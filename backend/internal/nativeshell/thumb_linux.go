package nativeshell

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// failName is the folder below thumbnails/fail where this app notes the
// files no thumbnailer could draw.
const failName = "cannoncommander"

// drawing lets only a few thumbnailers run at once, however many pictures
// a folder asks for.
var drawing = make(chan struct{}, 4)

// Thumbnail returns a PNG thumbnail of file at least px wide where there is
// one: from the cache that file managers share, or drawn by the
// thumbnailer the desktop has for the file's type and then put into that
// cache for the others.
func Thumbnail(ctx context.Context, file string, px int) ([]byte, error) {
	return current().thumbnail(ctx, file, px)
}

func (k *desk) thumbnail(ctx context.Context, file string, px int) ([]byte, error) {
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	root := k.ThumbnailDir()
	if !info.Mode().IsRegular() || strings.HasPrefix(file, root+"/") {
		return nil, ErrNoPicture
	}
	uri, mtime := xdg.FileURI(file), info.ModTime().Unix()
	name, size := xdg.ThumbnailName(uri), xdg.ThumbnailSize(px)
	if b, ok := cachedThumbnail(root, name, size, uri, mtime); ok {
		return b, nil
	}
	if failed(root, name, uri, mtime) {
		return nil, ErrNoPicture
	}
	// Without a sandbox no thumbnailer runs here, since the file may come
	// from anyone and thumbnailers have a long history of holes. The
	// desktop's thumbnail service runs them its own way.
	t, ok := k.thumbnailerFor(file)
	if !ok || k.sandbox.program == "" {
		return k.fromService(ctx, file, root, name, size, uri, mtime)
	}
	select {
	case drawing <- struct{}{}:
		defer func() { <-drawing }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Another request may have drawn it while this one waited.
	if b, ok := cachedThumbnail(root, name, size, uri, mtime); ok {
		return b, nil
	}
	text := map[string]string{"Thumb::URI": uri, "Thumb::MTime": strconv.FormatInt(mtime, 10)}
	b, err := k.draw(ctx, t, file, xdg.ThumbnailSizes[size].Edge)
	if err == nil {
		b, err = xdg.SetPNGText(b, text)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) || errors.Is(err, errNotDrawn) {
			_ = save(filepath.Join(root, "fail", failName, name), failure(text))
		}
		return nil, ErrNoPicture
	}
	// A cache that cannot be written still leaves the thumbnail to show.
	_ = save(filepath.Join(root, xdg.ThumbnailSizes[size].Dir, name), b)
	return b, nil
}

var errNotDrawn = errors.New("the thumbnailer wrote no PNG")

// fromService asks the desktop's thumbnail service for the thumbnail and
// reads it from the shared cache once the service is done.
func (k *desk) fromService(ctx context.Context, file, root, name string, size int, uri string, mtime int64) ([]byte, error) {
	if k.service == nil {
		return nil, ErrNoPicture
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := k.service(ctx, uri, k.db.TypeOf(file), xdg.ThumbnailSizes[size].Dir); err != nil {
		return nil, ErrNoPicture
	}
	if b, ok := cachedThumbnail(root, name, size, uri, mtime); ok {
		return b, nil
	}
	return nil, ErrNoPicture
}

// draw runs a thumbnailer on file in a sandbox and returns what it drew.
// It gives up after 20 seconds.
func (k *desk) draw(ctx context.Context, t xdg.Thumbnailer, file string, edge int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tmp, err := os.MkdirTemp("", "cc-thumbnail-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// The input keeps its extension, which some thumbnailers go by.
	inside := "/tmp/input" + filepath.Ext(file)
	args, err := t.Command(inside, "/tmp/thumbnail.png", edge)
	if err != nil {
		return nil, err
	}
	args = bwrapArgs(k.sandbox.program, k.sandbox.isolate, k.root, tmp, file, inside, args)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = tmp
	// A process group of its own lets a timeout end whatever the
	// thumbnailer started, too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(tmp, "thumbnail.png"))
	if err != nil {
		return nil, errNotDrawn
	}
	if _, err := xdg.PNGText(b); err != nil {
		return nil, errNotDrawn
	}
	return b, nil
}

// cachedThumbnail finds a thumbnail of the file in the folder of its size
// or of a larger one.
func cachedThumbnail(root, name string, size int, uri string, mtime int64) ([]byte, bool) {
	for _, s := range xdg.ThumbnailSizes[size:] {
		if b, err := os.ReadFile(filepath.Join(root, s.Dir, name)); err == nil && xdg.ThumbnailFits(b, uri, mtime) {
			return b, true
		}
	}
	return nil, false
}

// failed reports whether any program noted that it could not draw the file
// as it is.
func failed(root, name, uri string, mtime int64) bool {
	programs, _ := os.ReadDir(filepath.Join(root, "fail"))
	for _, p := range programs {
		if b, err := os.ReadFile(filepath.Join(root, "fail", p.Name(), name)); err == nil && xdg.ThumbnailFits(b, uri, mtime) {
			return true
		}
	}
	return false
}

// thumbnailerFor finds the thumbnailer for the nearest type file is of.
func (k *desk) thumbnailerFor(file string) (xdg.Thumbnailer, bool) {
	for _, m := range k.db.Lineage(k.db.TypeOf(file)) {
		for _, t := range k.thumbnailers {
			if slices.ContainsFunc(t.Mime, func(tm string) bool { return k.db.Canonical(tm) == m }) {
				return t, true
			}
		}
	}
	return xdg.Thumbnailer{}, false
}

// failure is the empty picture the specification asks for in place of a
// thumbnail that could not be drawn.
func failure(text map[string]string) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	out, _ := xdg.SetPNGText(b.Bytes(), text)
	return out
}

// save writes a thumbnail so no other program ever reads half of it, and
// only its owner reads it at all.
func save(file string, b []byte) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".cc-*.png")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), file)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}
