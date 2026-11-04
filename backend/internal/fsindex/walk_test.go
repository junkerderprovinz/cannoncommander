package fsindex_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex/fsindextest"
)

func tree(t *testing.T) (string, *ccfs.Resolver) {
	t.Helper()
	root := t.TempDir()
	for _, p := range []string{"filme/A/a.mkv", "filme/A b/b.mkv", "filme/.versteckt/c.mkv", "filme/z.txt", "musik/m.flac"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("1234"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return root, r
}

func walk(t *testing.T, r *ccfs.Resolver, dir string, skipHidden bool) ([]string, bool) {
	t.Helper()
	loc, err := r.Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	indexed, err := fsindex.Walk(context.Background(), loc, func(e fsindex.Entry) error {
		if skipHidden && e.Hidden {
			return fs.SkipDir
		}
		got = append(got, e.Path)
		if !e.Dir && e.Size != 4 {
			t.Errorf("%s has size %d", e.Path, e.Size)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, indexed
}

func TestWalkReadsTheDiskWithoutAnIndex(t *testing.T) {
	_, r := tree(t)
	got, indexed := walk(t, r, "/mnt/user/filme", true)
	want := []string{"/mnt/user/filme/A", "/mnt/user/filme/A/a.mkv", "/mnt/user/filme/A b", "/mnt/user/filme/A b/b.mkv", "/mnt/user/filme/z.txt"}
	if indexed || !slices.Equal(got, want) {
		t.Errorf("got %v (indexed %v), want %v", got, indexed, want)
	}
}

func TestWalkReadsTheIndexWhenItHoldsTheFolder(t *testing.T) {
	root, r := tree(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", root)
	// A file the index has not seen yet shows the walk did not look at the disk.
	if err := os.WriteFile(filepath.Join(root, "filme", "neu.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, indexed := walk(t, r, "/mnt/user/filme", true)
	if !indexed {
		t.Fatal("the walk went to the disk")
	}
	slices.Sort(got)
	want := []string{"/mnt/user/filme/A", "/mnt/user/filme/A b", "/mnt/user/filme/A b/b.mkv", "/mnt/user/filme/A/a.mkv", "/mnt/user/filme/z.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	all, _ := walk(t, r, "/mnt/user", false)
	if !slices.Contains(all, "/mnt/user/filme/.versteckt/c.mkv") || !slices.Contains(all, "/mnt/user/musik/m.flac") {
		t.Errorf("whole share: %v", all)
	}
}

func TestWalkFallsBackForAFolderTheIndexLacks(t *testing.T) {
	root, r := tree(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", filepath.Join(root, "musik"))
	if got, indexed := walk(t, r, "/mnt/user/filme/A", false); indexed || len(got) != 1 {
		t.Errorf("got %v (indexed %v)", got, indexed)
	}
}
