//go:build desktop

package trash

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

// fakeBin stands in for the trash of the system: it moves what it gets into
// a folder of its own and keeps the records in memory.
type fakeBin struct {
	dir string
	// noBin is a folder whose files have no trash, as a network drive.
	noBin string

	mu      sync.Mutex
	n       int
	entries map[string]sysbin.Entry
}

func newFakeBin(dir string) *fakeBin {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	return &fakeBin{dir: dir, entries: map[string]sysbin.Entry{}}
}

// of is the store's trash of the system, kept in f.
func (f *fakeBin) of(s *Store) systemBin { return &osBin{Store: s, bin: f} }

func (f *fakeBin) Put(p string) (sysbin.Entry, error) { return f.Adopt(p, p, time.Now()) }

func (f *fakeBin) Adopt(p, origin string, at time.Time) (sysbin.Entry, error) {
	if f.noBin != "" && strings.HasPrefix(p, f.noBin) {
		return sysbin.Entry{}, sysbin.ErrNoBin
	}
	info, err := os.Lstat(p)
	if err != nil {
		return sysbin.Entry{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	data := filepath.Join(f.dir, fmt.Sprint(f.n))
	if err := os.Rename(p, data); err != nil {
		return sysbin.Entry{}, err
	}
	e := sysbin.Entry{ID: fmt.Sprintf("f.%x", f.n), Name: filepath.Base(origin), Origin: origin, Path: data, Dir: info.IsDir(), Size: info.Size(), Deleted: at}
	f.entries[e.ID] = e
	return e, nil
}

func (f *fakeBin) List() ([]sysbin.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sysbin.Entry, 0, len(f.entries))
	for _, e := range f.entries {
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeBin) Get(id string) (sysbin.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok {
		return sysbin.Entry{}, sysbin.ErrNotFound
	}
	return e, nil
}

func (f *fakeBin) Restore(id, to string) error {
	e, err := f.Get(id)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return fs.ErrExist
	}
	if err := os.Rename(e.Path, to); err != nil {
		return err
	}
	return f.Forget(id)
}

func (f *fakeBin) Forget(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, id)
	return nil
}

// newSystemStore is a store as the desktop app has it on Windows: a drive as
// the source, and the system's trash on it.
func newSystemStore(t *testing.T) (*Store, string, *fakeBin) {
	t.Helper()
	root := t.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "C:", Mount: "/C", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	bin := newFakeBin(filepath.Join(root, "$Recycle.Bin"))
	s := openWith(v1.Deps{FS: fsys, DB: testutil.MainDB(t)}, bin.of)
	t.Cleanup(s.Close)
	return s, root, bin
}

func put(t *testing.T, s *Store, root, p, body string) Item {
	t.Helper()
	real := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(p, "/C/")))
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	loc, err := s.fs.Resolve(p)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.Put(context.Background(), "anna", loc)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func TestDesktopDeletesGoToTheSystemTrash(t *testing.T) {
	s, root, bin := newSystemStore(t)
	it := put(t, s, root, "/C/Bilder/a.jpg", "sand")
	if it.Origin != "/C/Bilder/a.jpg" || it.From != "/C/Bilder" || it.Size != 4 || it.Name != "a.jpg" {
		t.Fatalf("item %+v", it)
	}
	if _, err := os.Lstat(filepath.Join(root, "Bilder", "a.jpg")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the file is still there")
	}
	if len(bin.entries) != 1 {
		t.Fatalf("the system trash holds %d entries", len(bin.entries))
	}
	var rows int
	s.db.QueryRow(`SELECT COUNT(*) FROM trash_items`).Scan(&rows)
	if rows != 0 {
		t.Fatalf("%d rows in the app's own trash", rows)
	}
	items, err := s.List("ben")
	if err != nil || len(items) != 1 || items[0].Origin != "/C/Bilder/a.jpg" || items[0].ID != it.ID {
		t.Fatalf("list %+v, %v", items, err)
	}
	to, err := s.Restore(it.ID)
	if err != nil || to != "/C/Bilder/a.jpg" {
		t.Fatalf("restored to %s, %v", to, err)
	}
	if body, _ := os.ReadFile(filepath.Join(root, "Bilder", "a.jpg")); string(body) != "sand" {
		t.Fatalf("restored %q", body)
	}
}

func TestRestoringFromTheSystemTrashFindsAFreeName(t *testing.T) {
	s, root, _ := newSystemStore(t)
	it := put(t, s, root, "/C/Bilder/a.jpg", "alt")
	os.WriteFile(filepath.Join(root, "Bilder", "a.jpg"), []byte("neu"), 0o644)
	to, err := s.Restore(it.ID)
	if err != nil || to != "/C/Bilder/a (1).jpg" {
		t.Fatalf("restored to %s, %v", to, err)
	}
	if body, _ := os.ReadFile(filepath.Join(root, "Bilder", "a.jpg")); string(body) != "neu" {
		t.Fatal("the new file was replaced")
	}
}

func TestRestoringFromTheSystemTrashBuildsMissingFolders(t *testing.T) {
	s, root, _ := newSystemStore(t)
	it := put(t, s, root, "/C/x/y/a.txt", "a")
	if err := os.RemoveAll(filepath.Join(root, "x")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(it.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "x", "y", "a.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestDroppingASystemEntryRemovesItForGood(t *testing.T) {
	s, root, bin := newSystemStore(t)
	it := put(t, s, root, "/C/Ordner/a.txt", "a")
	data := bin.entries[it.ID].Path
	if err := s.Drop(context.Background(), nil, it.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(data); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the data is still in the trash")
	}
	if _, err := s.Get(it.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: %v", err)
	}
}

func TestADriveIsNoEntryOfTheSystemTrash(t *testing.T) {
	s, _, _ := newSystemStore(t)
	loc, _ := s.fs.Resolve("/C")
	if _, err := s.Put(context.Background(), "anna", loc); !errors.Is(err, ErrShareRoot) {
		t.Fatalf("got %v", err)
	}
}

func TestADriveWithoutTrashSaysSo(t *testing.T) {
	s, root, bin := newSystemStore(t)
	bin.noBin = filepath.Join(root, "Netz")
	os.MkdirAll(bin.noBin, 0o755)
	os.WriteFile(filepath.Join(bin.noBin, "a.txt"), nil, 0o644)
	loc, _ := s.fs.Resolve("/C/Netz/a.txt")
	_, err := s.Put(context.Background(), "anna", loc)
	if code, _ := v1.CodeOf(err); code != "noSystemTrash" {
		t.Fatalf("got %v, code %q", err, code)
	}
	if _, err := os.Stat(filepath.Join(bin.noBin, "a.txt")); err != nil {
		t.Fatal("the file went away")
	}
}

func TestEntriesFromOutsideTheSourcesStayForAdmins(t *testing.T) {
	s, _, bin := newSystemStore(t)
	elsewhere := filepath.Join(t.TempDir(), "fremd.txt")
	os.WriteFile(elsewhere, []byte("x"), 0o644)
	e, err := bin.Put(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.Get(e.ID)
	if err != nil || it.Origin != elsewhere || it.From != filepath.Dir(elsewhere) {
		t.Fatalf("item %+v, %v", it, err)
	}
	anna, admin := &users.User{Username: "anna"}, &users.User{Username: "root"}
	admin.Permissions.Admin = true
	if err := s.mayRestore(anna, it); err == nil {
		t.Fatal("anna may put it back outside her sources")
	}
	if err := s.mayRestore(admin, it); err != nil {
		t.Fatal(err)
	}
	if to, err := s.Restore(it.ID); err != nil || to != elsewhere {
		t.Fatalf("restored to %s, %v", to, err)
	}
}

func TestRemotesKeepTheAppsOwnTrash(t *testing.T) {
	root, remotes := t.TempDir(), t.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "C:", Mount: "/C", Root: root}, ccfs.Source{Name: "Fernziele", Mount: "/remotes", Root: remotes})
	if err != nil {
		t.Fatal(err)
	}
	bin := newFakeBin(filepath.Join(root, "$Recycle.Bin"))
	s := openWith(v1.Deps{FS: fsys, DB: testutil.MainDB(t)}, bin.of)
	t.Cleanup(s.Close)
	os.MkdirAll(filepath.Join(remotes, "gdrive"), 0o755)
	os.WriteFile(filepath.Join(remotes, "gdrive", "a.txt"), []byte("wolke"), 0o644)
	loc, _ := fsys.Resolve("/remotes/gdrive/a.txt")
	cloud, err := s.Put(context.Background(), "anna", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(bin.entries) != 0 || !strings.Contains(cloud.Bin, DirName) {
		t.Fatalf("a remote went to the system trash: %+v", cloud)
	}
	local := put(t, s, root, "/C/a.txt", "lokal")
	items, _ := s.List("anna")
	ids := []string{items[0].ID, items[1].ID}
	if len(items) != 2 || !slices.Contains(ids, cloud.ID) || !slices.Contains(ids, local.ID) {
		t.Fatalf("list %+v", items)
	}
	if to, err := s.Restore(cloud.ID); err != nil || to != "/remotes/gdrive/a.txt" {
		t.Fatalf("restored to %s, %v", to, err)
	}
}

func TestTheOldTrashMovesIntoTheSystemTrash(t *testing.T) {
	root := t.TempDir()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "C:", Mount: "/C", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	deps := v1.Deps{FS: fsys, DB: testutil.MainDB(t)}
	old := New(deps)
	a := put(t, old, root, "/C/Bilder/a.jpg", "a")
	put(t, old, root, "/C/Briefe/b.odt", "b")
	when := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	old.db.Exec(`UPDATE trash_items SET deleted_at = ? WHERE id = ?`, when.UnixMilli(), a.ID)
	old.Close()

	bin := newFakeBin(filepath.Join(root, "$Recycle.Bin"))
	s := openWith(deps, bin.of)
	t.Cleanup(s.Close)
	s.sys.takeOver()

	got := map[string]sysbin.Entry{}
	for _, e := range bin.entries {
		got[e.Origin] = e
	}
	for _, p := range []string{filepath.Join(root, "Bilder", "a.jpg"), filepath.Join(root, "Briefe", "b.odt")} {
		if _, ok := got[p]; !ok {
			t.Errorf("%s is not in the system trash: %+v", p, bin.entries)
		}
	}
	if e := got[filepath.Join(root, "Bilder", "a.jpg")]; !e.Deleted.Equal(when) {
		t.Errorf("deleted at %s, not %s", e.Deleted, when)
	}
	if _, err := os.Lstat(filepath.Join(root, DirName)); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the old trash folder is still there")
	}
	var rows int
	s.db.QueryRow(`SELECT COUNT(*) FROM trash_items`).Scan(&rows)
	if rows != 0 {
		t.Errorf("%d rows left", rows)
	}
	s.sys.takeOver()
	if len(bin.entries) != 2 {
		t.Errorf("a second round changed the trash: %d entries", len(bin.entries))
	}
}

// oldBin is a store on a drive at root with one record of the app's own
// trash, for the file name in the holder id, whatever lies there.
func oldBin(t *testing.T, root, id, name string) (*Store, *fakeBin) {
	t.Helper()
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "C:", Mount: "/C", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	deps := v1.Deps{FS: fsys, DB: testutil.MainDB(t)}
	if _, err := deps.DB.Exec(`INSERT INTO trash_items (id, owner, name, origin, bin, dir, size, files, deleted_at)
		VALUES (?, 'anna', ?, ?, ?, 0, 1, 1, 1)`, id, name, "/C/"+name, "/C/"+DirName+"/"+id+"/"+name); err != nil {
		t.Fatal(err)
	}
	bin := newFakeBin(filepath.Join(root, "$Recycle.Bin"))
	s := openWith(deps, bin.of)
	t.Cleanup(s.Close)
	return s, bin
}

func file(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLeftoversWithoutARecordStayWhereTheyAre(t *testing.T) {
	root := t.TempDir()
	lost := filepath.Join(root, DirName, "zz", "verloren.txt")
	file(t, lost, "?")
	s, bin := oldBin(t, root, "other", "x.txt")
	s.sys.takeOver()
	if len(bin.entries) != 0 {
		t.Fatalf("taken over: %+v", bin.entries)
	}
	if _, err := os.Stat(lost); err != nil {
		t.Fatal("the leftover went away")
	}
}

func TestATrashFolderThatIsALinkIsLeftAlone(t *testing.T) {
	root, victim := t.TempDir(), t.TempDir()
	file(t, filepath.Join(victim, "abc", "Dokumente.txt"), "privat")
	linkDir(t, victim, filepath.Join(root, DirName))
	s, bin := oldBin(t, root, "abc", "Dokumente.txt")
	s.sys.takeOver()
	if len(bin.entries) != 0 {
		t.Fatalf("taken over: %+v", bin.entries)
	}
	if _, err := os.Stat(filepath.Join(victim, "abc", "Dokumente.txt")); err != nil {
		t.Fatal("the file behind the link went away")
	}
}

func TestAHolderThatIsALinkIsLeftAlone(t *testing.T) {
	root, victim := t.TempDir(), t.TempDir()
	file(t, filepath.Join(victim, "Dokumente.txt"), "privat")
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir(t, victim, filepath.Join(root, DirName, "abc"))
	s, bin := oldBin(t, root, "abc", "Dokumente.txt")
	s.sys.takeOver()
	if len(bin.entries) != 0 {
		t.Fatalf("taken over: %+v", bin.entries)
	}
	if _, err := os.Stat(filepath.Join(victim, "Dokumente.txt")); err != nil {
		t.Fatal("the file behind the link went away")
	}
}
