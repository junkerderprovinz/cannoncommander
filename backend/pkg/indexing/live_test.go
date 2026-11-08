package indexing

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dbsql "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sql"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// liveIndex indexes a fresh folder holding files of four bytes each.
func liveIndex(t *testing.T, files ...string) (*Index, string) {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		put(t, root, f)
	}
	old := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = t.TempDir()
	db, _, err := dbsql.NewIndexDB(strings.ReplaceAll(t.Name(), "/", "_"), "OFF", 1000, 32, false)
	settings.Config.Server.CacheDir = old
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	idx := &Index{
		Source:              settings.Source{Name: "live", Path: root, Config: settings.SourceConfig{UseLogicalSize: true}},
		db:                  db,
		scanUpdatedPaths:    map[string]bool{},
		folderSizes:         map[string]uint64{},
		folderSizesUnsynced: map[string]struct{}{},
	}
	if err := idx.RefreshDirectory("/", true); err != nil {
		t.Fatal(err)
	}
	return idx, root
}

func put(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	hideDotted(t, root, rel)
}

func rows(t *testing.T, idx *Index) []string {
	t.Helper()
	r, err := idx.db.Query(`SELECT path FROM index_items WHERE source = ? ORDER BY path`, idx.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var out []string
	for r.Next() {
		var p string
		if err := r.Scan(&p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func wantRows(t *testing.T, idx *Index, want ...string) {
	t.Helper()
	if got := rows(t, idx); !slices.Equal(got, want) {
		t.Fatalf("index holds\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func wantSizes(t *testing.T, idx *Index, want map[string]uint64) {
	t.Helper()
	idx.folderSizesMu.RLock()
	defer idx.folderSizesMu.RUnlock()
	for k, v := range want {
		if got, ok := idx.folderSizes[k]; !ok || got != v {
			t.Errorf("size of %s is %d (known %v), want %d", k, got, ok, v)
		}
	}
	for k := range idx.folderSizes {
		if _, ok := want[k]; !ok {
			t.Errorf("size kept for %s", k)
		}
	}
}

func TestRefreshDropsWhatIsGone(t *testing.T) {
	idx, root := liveIndex(t, "a/x.txt", "a/z.txt", "a/sub/y.txt", "a/sub/deep/w.txt")
	if err := os.Remove(filepath.Join(root, "a", "x.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "a", "sub")); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshDirectory("/a/", false); err != nil {
		t.Fatal(err)
	}
	wantRows(t, idx, "/", "/a/", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 4, "/a/": 4})
}

func TestRefreshReadsNewFoldersInFull(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "b/q.txt")
	put(t, root, "a/n.txt")
	put(t, root, "a/neu/tief/f.txt")
	put(t, root, "a/neu/g.txt")
	if err := idx.RefreshDirectory("/a/", false); err != nil {
		t.Fatal(err)
	}
	wantRows(t, idx, "/", "/a/", "/a/n.txt", "/a/neu/", "/a/neu/g.txt", "/a/neu/tief/", "/a/neu/tief/f.txt", "/a/z.txt", "/b/", "/b/q.txt")
	wantSizes(t, idx, map[string]uint64{"/": 20, "/a/": 16, "/a/neu/": 8, "/a/neu/tief/": 4, "/b/": 4})
}

func TestRefreshUpdatesTheFolderTime(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt")
	then := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(filepath.Join(root, "a"), then, then); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshDirectory("/a/", false); err != nil {
		t.Fatal(err)
	}
	put(t, root, "a/neu.txt")
	if err := idx.RefreshDirectory("/a/", false); err != nil {
		t.Fatal(err)
	}
	var mtime int64
	if err := idx.db.QueryRow(`SELECT mod_time FROM index_items WHERE source = ? AND path = '/a/'`, idx.Name).Scan(&mtime); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if mtime != info.ModTime().Unix() || mtime == then.Unix() {
		t.Fatalf("folder time %d, on disk %d", mtime, info.ModTime().Unix())
	}
}

func TestAFolderThatIsGoneTakesItsRowsAlong(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "a/sub/y.txt", "a/sub/deep/w.txt")
	if err := os.RemoveAll(filepath.Join(root, "a", "sub")); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshDirectory("/a/sub/", false); err != nil {
		t.Fatal(err)
	}
	wantRows(t, idx, "/", "/a/", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 4, "/a/": 4})
}

func TestARefreshLeavesFoldersOutsideTheIndexAlone(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", ".versteckt/v.txt")
	put(t, root, ".versteckt/w.txt")
	if err := idx.RefreshDirectory("/.versteckt/", false); err != nil {
		t.Fatal(err)
	}
	wantRows(t, idx, "/", "/a/", "/a/z.txt")
}

func TestMovedFoldersKeepTheirRows(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "a/sub/y.txt", "a/sub/deep/w.txt", "b/q.txt")
	// A row the disk cannot give back shows the rows were moved, not read again.
	if _, err := idx.db.Exec(`UPDATE index_items SET type = 'marked' WHERE source = ? AND path = '/a/sub/deep/w.txt'`, idx.Name); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "a", "sub"), filepath.Join(root, "b", "neu")); err != nil {
		t.Fatal(err)
	}
	idx.Moved("/a/sub", "/b/neu")
	idx.RefreshDirectory("/a/", false)
	idx.RefreshDirectory("/b/", false)
	wantRows(t, idx, "/", "/a/", "/a/z.txt", "/b/", "/b/neu/", "/b/neu/deep/", "/b/neu/deep/w.txt", "/b/neu/y.txt", "/b/q.txt")
	var typ, parent string
	if err := idx.db.QueryRow(`SELECT type, parent_path FROM index_items WHERE source = ? AND path = '/b/neu/deep/w.txt'`, idx.Name).Scan(&typ, &parent); err != nil {
		t.Fatal(err)
	}
	if typ != "marked" || parent != "/b/neu/deep/" {
		t.Fatalf("moved row has type %q and parent %q", typ, parent)
	}
	wantSizes(t, idx, map[string]uint64{"/": 16, "/a/": 4, "/b/": 12, "/b/neu/": 8, "/b/neu/deep/": 4})
}

func TestRenamedFilesKeepTheirRows(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "a/x.txt")
	if err := os.Rename(filepath.Join(root, "a", "x.txt"), filepath.Join(root, "a", "y.txt")); err != nil {
		t.Fatal(err)
	}
	idx.Moved("/a/x.txt", "/a/y.txt")
	idx.RefreshDirectory("/a/", false)
	wantRows(t, idx, "/", "/a/", "/a/y.txt", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 8, "/a/": 8})
}

func TestMovesOutOfTheIndexDropTheRows(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "a/sub/y.txt", ".cc-trash/1/keep.txt")
	if err := os.Rename(filepath.Join(root, "a", "sub"), filepath.Join(root, ".cc-trash", "1", "sub")); err != nil {
		t.Fatal(err)
	}
	idx.Moved("/a/sub", "/.cc-trash/1/sub")
	idx.RefreshDirectory("/a/", false)
	wantRows(t, idx, "/", "/a/", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 4, "/a/": 4})
}

func TestMovesIntoTheIndexReadTheFolder(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", ".cc-trash/1/sub/y.txt")
	if err := os.Rename(filepath.Join(root, ".cc-trash", "1", "sub"), filepath.Join(root, "a", "sub")); err != nil {
		t.Fatal(err)
	}
	idx.Moved("/.cc-trash/1/sub", "/a/sub")
	idx.RefreshDirectory("/a/", false)
	wantRows(t, idx, "/", "/a/", "/a/sub/", "/a/sub/y.txt", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 8, "/a/": 8, "/a/sub/": 4})
}

func TestACreatedFolderIsReadInFull(t *testing.T) {
	idx, root := liveIndex(t, "a/z.txt", "a/sub/alt.txt")
	if err := os.RemoveAll(filepath.Join(root, "a", "sub")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "a/sub/neu/f.txt")
	idx.Created("/a/sub")
	idx.RefreshDirectory("/a/", false)
	wantRows(t, idx, "/", "/a/", "/a/sub/", "/a/sub/neu/", "/a/sub/neu/f.txt", "/a/z.txt")
	wantSizes(t, idx, map[string]uint64{"/": 8, "/a/": 8, "/a/sub/": 4, "/a/sub/neu/": 4})
}
