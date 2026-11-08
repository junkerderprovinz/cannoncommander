package fsapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ffmpeg"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/preview"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// thumbnailer draws the small pictures of the card views.
type thumbnailer interface {
	thumb(ctx context.Context, real string, info os.FileInfo, size string) ([]byte, error)
}

var errNoThumb = errors.New("no thumbnail for this file")

func (h handlers) thumb(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	size := q.Get("size")
	if size == "" {
		size = "small"
	}
	if size != "small" && size != "large" {
		return v1.Errorf(http.StatusBadRequest, "size is small or large")
	}
	u := v1.UserFrom(r)
	p := q.Get("path")
	if p == "" {
		return v1.Errorf(http.StatusBadRequest, "path is required")
	}
	loc, err := h.fs.Check(u, p, ccfs.View)
	if err != nil {
		return err
	}
	if loc.InArchive() {
		return v1.Errorf(http.StatusNotImplemented, "thumbnails from archives are not built yet")
	}
	if loc.Virtual() {
		return noThumb()
	}
	if err := h.fs.Follows(u, loc, ccfs.View); err != nil {
		return err
	}
	real := loc.Real
	info, err := os.Stat(real)
	if err != nil {
		return err
	}
	// A folder shows its poster, as the media libraries keep it.
	if info.IsDir() {
		_, pics, ok := peek(real)
		pic := pickPicture(pics)
		if !ok || pic == "" || !h.fs.Visible(u, h.seenFrom(loc), pic) {
			return noThumb()
		}
		pl, err := h.fs.Reach(u, path.Join(loc.Path, pic), ccfs.View)
		if err != nil {
			return noThumb()
		}
		real = pl.Real
		if info, err = os.Stat(real); err != nil || !info.Mode().IsRegular() {
			return noThumb()
		}
	}
	if !info.Mode().IsRegular() {
		return noThumb()
	}

	etag := strings.TrimSuffix(strongTag(info), `"`) + "-" + size + `"`
	hdr := w.Header()
	hdr.Set("ETag", etag)
	// A client that puts the file's mtime into v may keep the picture for
	// good, because a changed file gets a new URL.
	if q.Get("v") != "" {
		hdr.Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		hdr.Set("Cache-Control", "private, no-cache")
	}
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	data, err := h.thumbs.thumb(r.Context(), real, info, size)
	var ctype string
	if err == nil {
		var ok bool
		if ctype, ok = rasterType(data); !ok {
			err = errNoThumb
		}
	}
	if errors.Is(err, errNoThumb) {
		hdr.Del("ETag")
		hdr.Del("Cache-Control")
		return noThumb()
	}
	if err != nil {
		return err
	}
	hdr.Set("Content-Type", ctype)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", SandboxPolicy)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, err = w.Write(data)
	}
	return err
}

func noThumb() error { return v1.Errorf(http.StatusNotFound, "no thumbnail for this file") }

// rasterType names the picture format of data. The preview service hands
// small files and embedded covers back unchanged, so the bytes may be
// anything the file's owner wrote, whatever the name says.
func rasterType(data []byte) (string, bool) {
	// Go's sniffer takes AVIF for unknown binary data.
	if len(data) >= 12 && string(data[4:8]) == "ftyp" && (string(data[8:12]) == "avif" || string(data[8:12]) == "avis") {
		return "image/avif", true
	}
	switch t := http.DetectContentType(data); t {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/x-icon":
		return t, true
	}
	return "", false
}

func etagMatches(header, tag string) bool {
	for _, t := range strings.Split(header, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

// previewThumbs draws thumbnails with the preview service, which caches
// them on disk and knows images, videos, documents and album covers.
type previewThumbs struct{}

func (previewThumbs) thumb(ctx context.Context, real string, info os.FileInfo, size string) ([]byte, error) {
	if preview.GetService() == nil || settings.Config.Server.DisablePreviews {
		return nil, errNoThumb
	}
	file := iteminfo.ExtendedFileInfo{RealPath: real}
	file.Name, file.Size, file.ModTime = info.Name(), info.Size(), info.ModTime()
	file.DetectType(real, false)
	// The desktop app may still be looking for ffmpeg, which these need.
	if strings.HasPrefix(file.Type, "video/") || file.Type == "image/heic" || file.Type == "image/heif" {
		ffmpeg.Wait(ctx)
	}
	file.HasPreview = previewable(&file)
	if !file.HasPreview {
		return nil, errNoThumb
	}
	data, err := preview.GetPreviewForFile(ctx, file, size, "", 0)
	if errors.Is(err, preview.ErrUnsupportedMedia) || errors.Is(err, preview.ErrUnsupportedFormat) || errors.Is(err, preview.ErrTooManyPixels) || (err == nil && len(data) == 0) {
		return nil, errNoThumb
	}
	return data, err
}

// previewable follows the indexer's rules for which files get a preview,
// and loads the cover of an audio file, which the preview service expects
// to be there already.
func previewable(file *iteminfo.ExtendedFileInfo) bool {
	ext := strings.ToLower(filepath.Ext(file.Name))
	major, _, _ := strings.Cut(file.Type, "/")
	switch {
	case file.Size == 0:
		return false
	case file.Type == "image/heic" || file.Type == "image/heif":
		return file.Size <= iteminfo.LargeFileSizeThreshold && settings.CanConvertImage("heic")
	case major == "image":
		return file.Size <= iteminfo.LargeFileSizeThreshold
	case major == "video":
		return settings.CanConvertVideo(strings.TrimPrefix(ext, "."))
	case major == "audio":
		art := coverOf(file.RealPath)
		file.Metadata = &iteminfo.MediaMetadata{AlbumArt: art}
		return len(art) > 0
	}
	return iteminfo.HasDocConvertableExtension(file.Name, file.Type)
}

func coverOf(real string) []byte {
	f, err := os.Open(real)
	if err != nil {
		return nil
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil || m.Picture() == nil {
		return nil
	}
	return m.Picture().Data
}
