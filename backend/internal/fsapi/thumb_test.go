package fsapi

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/preview"
)

var startPreviews sync.Once

func withPreviews(t *testing.T) {
	t.Helper()
	startPreviews.Do(func() {
		dir, err := os.MkdirTemp("", "cc-thumbs-")
		if err != nil {
			t.Fatal(err)
		}
		_ = preview.StartPreviewGenerator(1, dir)
	})
}

// noise draws a picture that does not compress well, so it is large
// enough for the preview service to keep.
func noise(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(1, 2))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255})
		}
	}
	return img
}

func encode(t *testing.T, p string, enc func(*bytes.Buffer) error) {
	t.Helper()
	var buf bytes.Buffer
	if err := enc(&buf); err != nil {
		t.Fatal(err)
	}
	put(t, p, buf.Bytes())
}

func TestThumbOfAnImage(t *testing.T) {
	withPreviews(t)
	f := newFixture(t)
	encode(t, filepath.Join(f.share, "fotos", "2025", "bunt.png"), func(b *bytes.Buffer) error { return png.Encode(b, noise(64, 64)) })
	rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/bunt.png")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Body.Len() == 0 {
		t.Fatalf("thumb: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	tag := rec.Header().Get("ETag")
	if again := f.request(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/bunt.png", http.Header{"If-None-Match": {tag}}); again.Code != http.StatusNotModified {
		t.Errorf("unchanged: %d", again.Code)
	}
	if cc := f.get(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/bunt.png", "v", "123").Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("versioned cache control %q", cc)
	}
	if rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/bunt.png", "size", "huge"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown size: %d", rec.Code)
	}
}

func TestThumbOfAFolderShowsItsPoster(t *testing.T) {
	withPreviews(t)
	f := newFixture(t)
	encode(t, filepath.Join(f.share, "filme", "Sintel (2010)", "poster.jpg"), func(b *bytes.Buffer) error {
		return jpeg.Encode(b, noise(40, 60), &jpeg.Options{Quality: 95})
	})
	rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/filme/Sintel (2010)")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("folder thumb: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/musik/Podcasts"); rec.Code != http.StatusNotFound {
		t.Errorf("folder without a picture: %d", rec.Code)
	}
}

func TestThumbIsMissingForPlainFiles(t *testing.T) {
	withPreviews(t)
	f := newFixture(t)
	for _, p := range []string{"/mnt/user/downloads/setup-notes.md", "/mnt", "/mnt/user/fehlt.jpg"} {
		if rec := f.get(t, "jdp", "/fs/thumb", p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	if rec := f.get(t, "anna", "/fs/thumb", "/mnt/user/filme/Sintel (2010)/poster.jpg"); rec.Code != http.StatusForbidden {
		t.Errorf("outside the scope: %d", rec.Code)
	}
}
