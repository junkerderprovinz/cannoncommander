package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	dbsql "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sql"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// TestCheckFolderModtime_unchangedPreservesHasPreviewAndHidden verifies quick-scan touch updates
// do not clobber has_preview/hidden when folder modtime is unchanged.
func TestCheckFolderModtime_unchangedPreservesHasPreviewAndHidden(t *testing.T) {
	if indexDB == nil {
		var err error
		indexDB, _, err = dbsql.NewIndexDB("test_scanner_modtime", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}

	root := t.TempDir()
	photosDir := root + "/photos"
	if err := os.Mkdir(photosDir, 0o755); err != nil {
		t.Fatalf("mkdir photos: %v", err)
	}

	idx := &Index{
		ReducedIndex: ReducedIndex{},
		Source: settings.Source{
			Name: "test_modtime",
			Path: root,
		},
		db:               indexDB,
		scanUpdatedPaths: make(map[string]bool),
		folderSizes:      newFolderSizes(),
	}

	folderPath := "/photos/"
	now := time.Now()
	stored := &iteminfo.FileInfo{
		Path: folderPath,
		ItemInfo: iteminfo.ItemInfo{
			Name:       "photos",
			Type:       "directory",
			Size:       42,
			ModTime:    now.Add(-time.Hour),
			Hidden:     true,
			HasPreview: true,
		},
	}
	if err := idx.db.InsertItem("test_modtime", folderPath, stored); err != nil {
		t.Fatalf("InsertItem: %v", err)
	}
	idx.SetFolderSize(folderPath, 42)

	s := &Scanner{
		scanPath: "/",
		idx:      idx,
	}
	s.withStatsLock(func() {
		s.lastScanned = time.Now()
	})
	s.batchItems = make([]*iteminfo.FileInfo, 0, idx.db.BatchSize)

	changed, err := s.checkFolderModtime(folderPath)
	if err != nil {
		t.Fatalf("checkFolderModtime: %v", err)
	}
	if changed {
		t.Fatal("expected unchanged folder")
	}
	s.flushBatch()

	got, ok := idx.GetReducedMetadata(folderPath, true)
	if !ok || got == nil {
		t.Fatal("expected folder metadata after touch update")
	}
	if !got.HasPreview {
		t.Error("HasPreview should be preserved on unchanged modtime touch update")
	}
	if !got.Hidden {
		t.Error("Hidden should be preserved on unchanged modtime touch update")
	}
}

// quickScanIndex reads a tree with a full pass of the scanner for /cloud/
// and dates every folder back, so a quick scan sees only what changes next.
func quickScanIndex(t *testing.T, folders ...string) (*Index, *Scanner) {
	t.Helper()
	root := t.TempDir()
	for _, f := range folders {
		put(t, root, "cloud/"+f+"/file.txt")
	}
	idx := benchIndex(t, root)
	s := idx.createScanner("/cloud/")
	if err := s.runFullScanChild(); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			err = os.Chtimes(p, past, past)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	s.lastScanned = time.Now().Add(-time.Hour)
	return idx, s
}

func TestAQuickScanReadsAChangedFolderDeepDown(t *testing.T) {
	idx, s := quickScanIndex(t, "a/b/c", "d")
	put(t, idx.Path, "cloud/a/b/c/new.txt")
	if err := s.runQuickScanChild(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/cloud/a/b/c/new.txt", "/cloud/a/b/c/file.txt", "/cloud/d/file.txt"} {
		if got, err := idx.db.GetItem(idx.Name, p); err != nil || got == nil {
			t.Errorf("%s after the quick scan: %+v, %v", p, got, err)
		}
	}
	if size, ok := idx.GetFolderSize("/cloud/a/b/c/"); !ok || size != 8 {
		t.Errorf("size of the changed folder %d (known %v), want 8", size, ok)
	}
}

func TestTheRootQuickScanReadsTheFoldersInTheRoot(t *testing.T) {
	idx, _ := quickScanIndex(t, "a")
	s := idx.createScanner("/")
	s.lastScanned = time.Now().Add(-time.Hour)
	put(t, idx.Path, "cloud/new.txt")
	if err := s.runQuickScanRoot(); err != nil {
		t.Fatal(err)
	}
	if got, err := idx.db.GetItem(idx.Name, "/cloud/new.txt"); err != nil || got == nil {
		t.Errorf("new file after the quick scan: %+v, %v", got, err)
	}
}
