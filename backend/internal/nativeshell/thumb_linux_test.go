package nativeshell

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// cached puts a thumbnail of file into the cache folder dir, made when the
// file was last changed at mtime.
func (f *fixture) cached(dir, file string, mtime int64, w int) {
	f.t.Helper()
	uri := xdg.FileURI(file)
	b, err := xdg.SetPNGText(pngBytes(f.t, w, w), map[string]string{"Thumb::URI": uri, "Thumb::MTime": strconv.FormatInt(mtime, 10)})
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(filepath.Join(f.home, ".cache", "thumbnails", dir, xdg.ThumbnailName(uri)), string(b))
}

func mtime(t *testing.T, file string) int64 {
	t.Helper()
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime().Unix()
}

func size(t *testing.T, b []byte) int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img.Bounds().Dx()
}

func TestAThumbnailComesFromTheCacheWhileItFitsTheFile(t *testing.T) {
	f := newFixture(t)
	files := f.files("a.png", "b.png", "c.png")
	f.cached("normal", files[0], mtime(t, files[0]), 128)
	f.cached("large", files[1], mtime(t, files[1]), 256)
	f.cached("normal", files[2], mtime(t, files[2])-1, 128)
	k := f.desk()
	ctx := context.Background()
	if b, err := k.thumbnail(ctx, files[0], 100); err != nil || size(t, b) != 128 {
		t.Errorf("normal: %v", err)
	}
	if b, err := k.thumbnail(ctx, files[1], 100); err != nil || size(t, b) != 256 {
		t.Errorf("a larger one: %v", err)
	}
	if _, err := k.thumbnail(ctx, files[0], 200); !errors.Is(err, ErrNoPicture) {
		t.Errorf("a smaller one for a larger size: %v", err)
	}
	if _, err := k.thumbnail(ctx, files[2], 100); !errors.Is(err, ErrNoPicture) {
		t.Errorf("an outdated one: %v", err)
	}
	if _, err := k.thumbnail(ctx, f.files("photos/")[0], 100); !errors.Is(err, ErrNoPicture) {
		t.Errorf("a folder: %v", err)
	}
	if _, err := k.thumbnail(ctx, filepath.Join(f.work, "gone.png"), 100); err == nil {
		t.Error("a missing file has a thumbnail")
	}
}

// thumbnailer installs a thumbnailer for PNGs whose program writes its
// arguments to a log and then runs body.
func (f *fixture) thumbnailer(body string) (log string) {
	log = filepath.Join(f.home, "thumbnailer.log")
	f.bwrap = f.script(filepath.Join(f.home, "bin", "bwrap"), fakeBwrap)
	program := f.script(filepath.Join(f.home, "bin", "thumbnail me"), "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]' \"$a\" >> "+shellQuote(log)+"; done\necho >> "+shellQuote(log)+"\n"+body+"\n")
	f.write(filepath.Join(f.system, "thumbnailers", "test.thumbnailer"), "[Thumbnailer Entry]\nExec=\""+program+"\" -s %s %i %o %u\nMimeType=image/png;\n")
	return log
}

func TestAThumbnailerDrawsAndTheCacheKeepsIt(t *testing.T) {
	f := newFixture(t)
	source := f.write(filepath.Join(f.home, "drawn.png"), string(pngBytes(t, 256, 256)))
	log := f.thumbnailer(`cp "` + source + `" "$4"`)
	file := f.files("a;rm -rf ~.png")[0]
	k := f.desk()
	b, err := k.thumbnail(context.Background(), file, 200)
	if err != nil {
		t.Fatal(err)
	}
	uri := xdg.FileURI(file)
	if text, err := xdg.PNGText(b); err != nil || text["Thumb::URI"] != uri || text["Thumb::MTime"] != strconv.FormatInt(mtime(t, file), 10) || size(t, b) != 256 {
		t.Errorf("got %v %v", text, err)
	}
	saved := filepath.Join(f.home, ".cache", "thumbnails", "large", xdg.ThumbnailName(uri))
	info, err := os.Stat(saved)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved %v %v", info, err)
	}
	if dir, err := os.Stat(filepath.Dir(saved)); err != nil || dir.Mode().Perm() != 0o700 {
		t.Errorf("folder %v %v", dir, err)
	}
	logged, _ := os.ReadFile(log)
	// The thumbnailer sees the paths inside the sandbox, which the stand-in
	// for bwrap turned back into its private folder.
	args := regexp.MustCompile(`^\[-s\]\[256\]\[/[^\]]+/input\.png\]\[/[^\]]+/thumbnail\.png\]\[file:///tmp/input\.png\]\n$`)
	if !args.Match(logged) {
		t.Errorf("arguments %q", logged)
	}
	sandbox, _ := os.ReadFile(filepath.Join(f.home, "bin", "bwrap.log"))
	if !strings.Contains(string(sandbox), "\n--ro-bind\n"+file+"\n/tmp/input.png\n") || !strings.Contains(string(sandbox), "\n--clearenv\n") {
		t.Errorf("sandbox %q", sandbox)
	}
	if again, err := k.thumbnail(context.Background(), file, 200); err != nil || !bytes.Equal(again, b) {
		t.Errorf("again: %v", err)
	}
	if logged, _ := os.ReadFile(log); strings.Count(string(logged), "\n") != 1 {
		t.Errorf("the thumbnailer ran again: %q", logged)
	}
}

func TestAFailedThumbnailIsNotTriedAgain(t *testing.T) {
	f := newFixture(t)
	log := f.thumbnailer("exit 1")
	files := f.files("broken.png", "known.png")
	k := f.desk()
	ctx := context.Background()
	if _, err := k.thumbnail(ctx, files[0], 100); !errors.Is(err, ErrNoPicture) {
		t.Fatalf("got %v", err)
	}
	if _, err := k.thumbnail(ctx, files[0], 100); !errors.Is(err, ErrNoPicture) {
		t.Fatalf("again: %v", err)
	}
	if logged, _ := os.ReadFile(log); strings.Count(string(logged), "\n") != 1 {
		t.Errorf("the thumbnailer ran %q", logged)
	}
	if _, err := os.Stat(filepath.Join(f.home, ".cache", "thumbnails", "fail", failName, xdg.ThumbnailName(xdg.FileURI(files[0])))); err != nil {
		t.Error(err)
	}
	// Another program's note counts as well.
	f.cached(filepath.Join("fail", "gnome-thumbnail-factory"), files[1], mtime(t, files[1]), 1)
	if _, err := k.thumbnail(ctx, files[1], 100); !errors.Is(err, ErrNoPicture) {
		t.Fatalf("known: %v", err)
	}
	if logged, _ := os.ReadFile(log); strings.Count(string(logged), "\n") != 1 {
		t.Errorf("the thumbnailer ran for a known failure %q", logged)
	}
}

func TestAThumbnailerThatHangsIsStopped(t *testing.T) {
	f := newFixture(t)
	f.thumbnailer("sleep 30")
	file := f.files("slow.png")[0]
	k := f.desk()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	began := time.Now()
	if _, err := k.thumbnail(ctx, file, 100); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v", err)
	}
	if time.Since(began) > 5*time.Second {
		t.Errorf("took %v", time.Since(began))
	}
	if _, err := os.Stat(filepath.Join(f.home, ".cache", "thumbnails", "fail")); err == nil {
		t.Error("a timeout was noted as a failure")
	}
}

func TestAFileWithoutAThumbnailerHasNoThumbnail(t *testing.T) {
	f := newFixture(t)
	file := f.files("notes.txt")[0]
	f.thumbnailer("exit 0")
	if _, err := f.desk().thumbnail(context.Background(), file, 100); !errors.Is(err, ErrNoPicture) {
		t.Errorf("got %v", err)
	}
	empty := f.files("empty.png")[0]
	if _, err := f.desk().thumbnail(context.Background(), empty, 100); !errors.Is(err, ErrNoPicture) {
		t.Errorf("nothing drawn: %v", err)
	}
}
