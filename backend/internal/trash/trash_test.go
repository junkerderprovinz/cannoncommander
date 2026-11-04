package trash

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

func TestEveryShareHasItsOwnBin(t *testing.T) {
	fsys, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: t.TempDir()},
		ccfs.Source{Name: "disk1", Mount: "/mnt/disk1", Root: t.TempDir()},
		ccfs.Source{Name: "gdrive", Mount: "/mnt/remotes/gdrive", Root: t.TempDir()},
	)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		"/mnt/user/fotos/a.jpg":         "/mnt/user/fotos/.cc-trash",
		"/mnt/user/fotos/2025/a.jpg":    "/mnt/user/fotos/.cc-trash",
		"/mnt/disk1/filme/x.mkv":        "/mnt/disk1/filme/.cc-trash",
		"/mnt/remotes/gdrive/doc/a.pdf": "/mnt/remotes/gdrive/.cc-trash",
		"/mnt/remotes/gdrive/a.pdf":     "/mnt/remotes/gdrive/.cc-trash",
	} {
		loc, err := fsys.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := BinFor(loc); err != nil || got != want {
			t.Errorf("%s: %s, %v", p, got, err)
		}
	}
	for _, p := range []string{"/mnt/user/fotos", "/mnt/remotes/gdrive", "/mnt", "/mnt/user/fotos/.cc-trash/x/a.jpg"} {
		loc, err := fsys.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := BinFor(loc); err == nil {
			t.Errorf("%s may go to the trash at %s", p, got)
		}
	}
}

func newStore(t *testing.T) (*Store, string, *ccfs.Resolver) {
	t.Helper()
	root := t.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	s := New(v1.Deps{FS: fsys, DB: testutil.MainDB(t)})
	t.Cleanup(s.Close)
	return s, root, fsys
}

func TestEntriesExpireAfterTheirDays(t *testing.T) {
	s, root, fsys := newStore(t)
	for _, name := range []string{"alt.txt", "neu.txt"} {
		if err := os.MkdirAll(filepath.Join(root, "fotos"), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "fotos", name), []byte(name), 0o666); err != nil {
			t.Fatal(err)
		}
		loc, _ := fsys.Resolve("/mnt/user/fotos/" + name)
		it, err := s.Put(context.Background(), "anna", loc)
		if err != nil {
			t.Fatal(err)
		}
		if name == "alt.txt" {
			back := time.Now().Add(-8 * 24 * time.Hour).UnixMilli()
			if _, err := s.db.Exec(`UPDATE trash_items SET deleted_at = ? WHERE id = ?`, back, it.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.SetDays(0); err == nil {
		t.Fatal("zero days were accepted")
	}
	if err := s.SetDays(7); err != nil {
		t.Fatal(err)
	}
	items, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "neu.txt" {
		t.Fatalf("left: %+v", items)
	}
	if items[0].ExpiresAt.Sub(items[0].DeletedAt) != 7*24*time.Hour {
		t.Fatal("expiry does not follow the days")
	}
	bins, _ := os.ReadDir(filepath.Join(root, "fotos", DirName))
	if len(bins) != 1 {
		t.Fatalf("bin holds %d entries", len(bins))
	}
}

func TestEntriesThatVanishedFromTheBinAreDropped(t *testing.T) {
	s, root, fsys := newStore(t)
	if err := os.MkdirAll(filepath.Join(root, "fotos", "a"), 0o777); err != nil {
		t.Fatal(err)
	}
	loc, _ := fsys.Resolve("/mnt/user/fotos/a")
	it, err := s.Put(context.Background(), "anna", loc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "fotos", DirName)); err != nil {
		t.Fatal(err)
	}
	if items, _ := s.List("anna"); len(items) != 0 {
		t.Fatalf("listed %+v", items)
	}
	if _, err := s.Get(it.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: %v", err)
	}
}
