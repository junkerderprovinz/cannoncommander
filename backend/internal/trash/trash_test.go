package trash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

func TestEveryShareHasItsOwnBin(t *testing.T) {
	fsys, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: t.TempDir()},
		ccfs.Source{Name: "disk1", Mount: "/mnt/disk1", Root: t.TempDir()},
		ccfs.Source{Name: "Fernziele", Mount: "/remotes", Root: t.TempDir()},
	)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		"/mnt/user/fotos/a.jpg":      "/mnt/user/fotos/.cc-trash",
		"/mnt/user/fotos/2025/a.jpg": "/mnt/user/fotos/.cc-trash",
		"/mnt/disk1/filme/x.mkv":     "/mnt/disk1/filme/.cc-trash",
		"/remotes/gdrive/doc/a.pdf":  "/remotes/gdrive/.cc-trash",
		"/remotes/gdrive/a.pdf":      "/remotes/gdrive/.cc-trash",
	} {
		loc, err := fsys.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := BinFor(loc); err != nil || got != want {
			t.Errorf("%s: %s, %v", p, got, err)
		}
	}
	for _, p := range []string{"/mnt/user/fotos", "/remotes/gdrive", "/remotes", "/mnt", "/mnt/user/fotos/.cc-trash/x/a.jpg"} {
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

func TestABurstOfChangesIsToldAtOnceAndAgainAtItsEnd(t *testing.T) {
	root := t.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(hub.Options{})
	s := New(v1.Deps{FS: fsys, DB: testutil.MainDB(t), Hub: h})
	t.Cleanup(s.Close)
	client, _, _ := h.Connect("tab", "anna", 0)
	if err := os.MkdirAll(filepath.Join(root, "fotos"), 0o755); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	for i := range 100 {
		name := fmt.Sprintf("%d.jpg", i)
		if err := os.WriteFile(filepath.Join(root, "fotos", name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		loc, err := fsys.Resolve("/mnt/user/fotos/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Put(context.Background(), "anna", loc); err != nil {
			t.Fatal(err)
		}
	}
	took := time.Since(began)
	n := 0
	for len(client.Events) > 0 {
		<-client.Events
		n++
	}
	if most := int(took/tellEvery) + 1; n == 0 || n > most {
		t.Fatalf("%d changes told for a burst of 100 in %s, at most %d expected", n, took, most)
	}
	select {
	case <-client.Events:
	case <-time.After(5 * time.Second):
		t.Fatal("the end of the burst was not told")
	}
}

func TestPickFindsEachEntryOnceAndKeepsToItsOwner(t *testing.T) {
	s, _, _ := newStore(t)
	for _, row := range [][2]string{{"a", "anna"}, {"b", "anna"}, {"c", "ben"}} {
		if _, err := s.db.Exec(`INSERT INTO trash_items (id, owner, name, origin, bin, dir, size, files, deleted_at)
			VALUES (?, ?, 'x', '/mnt/user/x', '/mnt/user/.cc-trash/x', 0, 1, 1, ?)`, row[0], row[1], time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	h := handlers{s: s}
	anna, admin := &users.User{Username: "anna"}, &users.User{Username: "root"}
	admin.Permissions.Admin = true
	got, err := h.pick(anna, []string{"b", "a", "b"})
	if err != nil || len(got) != 2 || got[0].ID != "b" || got[1].ID != "a" {
		t.Errorf("anna picked %+v, %v", got, err)
	}
	if _, err := h.pick(anna, []string{"a", "c"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("the entry of another user: %v", err)
	}
	if _, err := h.pick(admin, []string{"a", "nichts"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("an entry that does not exist: %v", err)
	}
	if got, err := h.pick(admin, []string{"c", "a"}); err != nil || len(got) != 2 {
		t.Errorf("admin picked %+v, %v", got, err)
	}
}

func BenchmarkPickTwentyThousandEntries(b *testing.B) {
	s := New(v1.Deps{FS: &ccfs.Resolver{}, DB: testutil.MainDB(b)})
	b.Cleanup(s.Close)
	tx, err := s.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	ids := make([]string, 20000)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
		if _, err := tx.Exec(`INSERT INTO trash_items (id, owner, name, origin, bin, dir, size, files, deleted_at)
			VALUES (?, 'anna', 'x', '/mnt/user/x', '/mnt/user/.cc-trash/x', 0, 1, 1, 1)`, ids[i]); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	h, anna := handlers{s: s}, &users.User{Username: "anna"}
	for b.Loop() {
		if _, err := h.pick(anna, ids); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrepareTwentyThousandPaths(b *testing.B) {
	root := b.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "fotos"), 0o755); err != nil {
		b.Fatal(err)
	}
	paths := make([]string, 20000)
	for i := range paths {
		name := fmt.Sprintf("%d.jpg", i)
		if err := os.WriteFile(filepath.Join(root, "fotos", name), nil, 0o644); err != nil {
			b.Fatal(err)
		}
		paths[i] = "/mnt/user/fotos/" + name
	}
	raw, _ := json.Marshal(PathsParams{Paths: paths})
	anna := &users.User{Username: "anna", BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: users.SourceFilePermissions{View: true, Delete: true}}}}
	for b.Loop() {
		if _, err := prepareTrash(context.Background(), v1.Deps{FS: fsys}, anna, raw); err != nil {
			b.Fatal(err)
		}
	}
}

type someApp struct{}

func (someApp) AutoUpdate() bool         { return false }
func (someApp) SetAutoUpdate(bool) error { return nil }
func (someApp) UpdateReady() string      { return "" }
func (someApp) Restart()                 {}

func TestBelowTheRootOnlyTheHomeFolderHasABin(t *testing.T) {
	desktop.Use(someApp{}, desktop.Options{Home: "/home/jdp"})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Computer", Mount: "/", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := fsys.Resolve("/home/jdp/Bilder/strand.jpg")
	if got, err := BinFor(loc); err != nil || got != "/home/jdp/.cc-trash" {
		t.Errorf("in the home folder: %s, %v", got, err)
	}
	for _, p := range []string{"/etc/hosts", "/home/jdp", "/home/anna/x"} {
		loc, _ := fsys.Resolve(p)
		if _, err := BinFor(loc); !errors.Is(err, ErrNoBin) {
			t.Errorf("%s: %v", p, err)
		}
	}
}
