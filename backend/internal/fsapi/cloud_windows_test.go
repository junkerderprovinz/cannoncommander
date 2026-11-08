package fsapi

import (
	"bytes"
	"image/png"
	"net/http"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAFileInTheCloudShowsItsStateAndGetsNoThumbnail(t *testing.T) {
	withPreviews(t)
	f := newFixture(t)
	dir := filepath.Join(f.share, "fotos", "2025")
	for _, n := range []string{"wolke.png", "hier.png"} {
		encode(t, filepath.Join(dir, n), func(b *bytes.Buffer) error { return png.Encode(b, noise(64, 64)) })
	}
	setAttrs(t, filepath.Join(dir, "wolke.png"), windows.FILE_ATTRIBUTE_OFFLINE)
	setAttrs(t, filepath.Join(dir, "hier.png"), windows.FILE_ATTRIBUTE_ARCHIVE|attrPinned)

	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos/2025"))
	if got := byName(t, l.Items, "wolke.png").Cloud; got != "online" {
		t.Errorf("online-only file: %q", got)
	}
	if got := byName(t, l.Items, "hier.png").Cloud; got != "pinned" {
		t.Errorf("kept file: %q", got)
	}
	if rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/wolke.png"); rec.Code != http.StatusNotFound {
		t.Errorf("thumbnail of the online-only file: %d", rec.Code)
	}
	if rec := f.get(t, "jdp", "/fs/thumb", "/mnt/user/fotos/2025/hier.png"); rec.Code != http.StatusOK {
		t.Errorf("thumbnail of the kept file: %d", rec.Code)
	}
}
