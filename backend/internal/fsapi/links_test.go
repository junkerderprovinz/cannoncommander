package fsapi

import (
	"bytes"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// hideFor hides p from one user.
type hideFor struct {
	user, path string
	effect     ccfs.Effect
}

func (r hideFor) Effect(u *users.User, p string) ccfs.Effect {
	if u.Username == r.user && (p == r.path || strings.HasPrefix(p, r.path+"/")) {
		return r.effect
	}
	return ccfs.Open
}

func linkedFixture(t *testing.T, rule hideFor) fixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks")
	}
	f := newFixtureWith(t, rule)
	write(t, filepath.Join(f.share, "fotos", "geheim", "x.txt"), 7)
	write(t, filepath.Join(f.share, "fotos", "frei", "a.txt"), 1)
	return f
}

func TestALinkDoesNotLeadPastAHideRule(t *testing.T) {
	f := linkedFixture(t, hideFor{"anna", "/mnt/user/fotos/geheim", ccfs.Hidden})
	if err := os.Symlink(filepath.Join(f.share, "fotos", "geheim"), filepath.Join(f.share, "fotos", "frei", "tor")); err != nil {
		t.Fatal(err)
	}
	const through = "/mnt/user/fotos/frei/tor"
	if rec := f.get(t, "anna", "/fs/raw", "/mnt/user/fotos/geheim/x.txt"); rec.Code != http.StatusForbidden {
		t.Fatalf("direct read: %d", rec.Code)
	}
	for _, c := range []struct{ endpoint, path string }{
		{"/fs/raw", through + "/x.txt"},
		{"/fs/text", through + "/x.txt"},
		{"/fs/stat", through + "/x.txt"},
		{"/fs/list", through},
		{"/fs/tree", through},
	} {
		if rec := f.get(t, "anna", c.endpoint, c.path); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d", c.endpoint, c.path, rec.Code)
		}
	}
}

func TestAListThroughALinkGoesByTheRulesOfWhereItLeads(t *testing.T) {
	f := linkedFixture(t, hideFor{"anna", "/mnt/user/fotos/geheim", ccfs.Hidden})
	if err := os.Symlink(filepath.Join(f.share, "fotos"), filepath.Join(f.share, "fotos", "frei", "oben")); err != nil {
		t.Fatal(err)
	}
	rec := f.get(t, "anna", "/fs/list", "/mnt/user/fotos/frei/oben")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	for _, n := range names(decode[Listing](t, rec).Items) {
		if n == "geheim" {
			t.Error("the hidden folder shows up through the link")
		}
	}
}

func TestALinkDoesNotLeadOutOfTheScope(t *testing.T) {
	f := linkedFixture(t, hideFor{})
	if err := os.Symlink(filepath.Join(f.share, "filme"), filepath.Join(f.share, "fotos", "frei", "tor")); err != nil {
		t.Fatal(err)
	}
	if rec := f.get(t, "anna", "/fs/list", "/mnt/user/fotos/frei/tor"); rec.Code != http.StatusForbidden {
		t.Errorf("list through the link: %d", rec.Code)
	}
	if rec := f.get(t, "anna", "/fs/raw", "/mnt/user/fotos/frei/tor/Sintel (2010)/poster.jpg"); rec.Code != http.StatusForbidden {
		t.Errorf("read through the link: %d", rec.Code)
	}
}

func TestAFolderPosterBehindALinkStaysHidden(t *testing.T) {
	f := linkedFixture(t, hideFor{"anna", "/mnt/user/fotos/geheim", ccfs.Hidden})
	withPreviews(t)
	encode(t, filepath.Join(f.share, "fotos", "geheim", "bild.png"), func(b *bytes.Buffer) error { return png.Encode(b, noise(64, 64)) })
	if err := os.Symlink(filepath.Join(f.share, "fotos", "geheim", "bild.png"), filepath.Join(f.share, "fotos", "frei", "poster.png")); err != nil {
		t.Fatal(err)
	}
	if rec := f.get(t, "anna", "/fs/thumb", "/mnt/user/fotos/frei", "size", "small"); rec.Code == http.StatusOK {
		t.Errorf("thumb of a folder whose poster is a link into a hidden place: %d", rec.Code)
	}
}

func TestAnArchiveLeavesOutALinkIntoAPlaceItsUserMayNotRead(t *testing.T) {
	f := linkedFixture(t, hideFor{"anna", "/mnt/user/fotos/geheim", ccfs.Hidden})
	put(t, filepath.Join(f.share, "filme", "lohn.txt"), []byte("lohn"))
	if err := os.Symlink(filepath.Join(f.share, "filme", "lohn.txt"), filepath.Join(f.share, "fotos", "frei", "scope.txt")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.share, "fotos", "geheim", "y.txt"), []byte("geheim"))
	if err := os.Symlink(filepath.Join(f.share, "fotos", "geheim", "y.txt"), filepath.Join(f.share, "fotos", "frei", "regel.txt")); err != nil {
		t.Fatal(err)
	}
	rec := f.archive(t, "anna", archiveRequest{Paths: []string{"/mnt/user/fotos/frei"}})
	entries := zipEntries(t, rec)
	for _, name := range []string{"frei/scope.txt", "frei/regel.txt"} {
		if _, ok := entries[name]; ok {
			t.Errorf("%s came along", name)
		}
	}
}

func TestAnArchiveThroughALinkedFolderGoesByTheRulesWhereItLeads(t *testing.T) {
	f := linkedFixture(t, hideFor{"anna", "/mnt/user/fotos/album/geheim", ccfs.Hidden})
	put(t, filepath.Join(f.share, "fotos", "album", "offen.txt"), []byte("offen"))
	put(t, filepath.Join(f.share, "fotos", "album", "geheim", "y.txt"), []byte("geheim"))
	if err := os.Symlink(filepath.Join(f.share, "fotos"), filepath.Join(f.share, "fotos", "frei", "tor")); err != nil {
		t.Fatal(err)
	}
	rec := f.archive(t, "anna", archiveRequest{Paths: []string{"/mnt/user/fotos/frei/tor/album"}})
	entries := zipEntries(t, rec)
	if _, ok := entries["album/offen.txt"]; !ok {
		t.Errorf("the open file is missing: %v", keys(entries))
	}
	if _, ok := entries["album/geheim/y.txt"]; ok {
		t.Error("the hidden file came along")
	}
}

func TestTheBinCannotBeReachedByPath(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.share, "fotos", ".cc-trash", "id", "x.txt"), 3)
	for _, endpoint := range []string{"/fs/list", "/fs/raw", "/fs/stat"} {
		for _, p := range []string{"/mnt/user/fotos/.cc-trash", "/mnt/user/fotos/.cc-trash/id/x.txt"} {
			if rec := f.get(t, "anna", endpoint, p); rec.Code < 400 {
				t.Errorf("%s %s: %d", endpoint, p, rec.Code)
			}
		}
	}
}
