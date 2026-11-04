package indexing

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitForPasses(t *testing.T, idx *Index, n uint64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for idx.FullPasses() < n {
		if time.Now().After(deadline) {
			t.Fatalf("still %d passes, want %d", idx.FullPasses(), n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRequestFullScanRunsAPassThroughTheScheduler(t *testing.T) {
	idx := newRestoreTestIndex(t, "rescan", []string{"a", "b"}, persistedScanners("/", "/a/", "/b/"))
	idx.setupMultiScanner(false)
	idx.mu.RLock()
	bootstrap := idx.startupScanPending
	idx.mu.RUnlock()
	if bootstrap {
		waitForPasses(t, idx, 1)
	}
	if err := os.WriteFile(filepath.Join(idx.Path, "a", "neu.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := idx.FullPasses()
	if err := idx.RequestFullScan(); err != nil {
		t.Fatal(err)
	}
	waitForPasses(t, idx, before+1)
	if got, err := idx.db.GetItem("rescan", "/a/neu.txt"); err != nil || got == nil {
		t.Errorf("new file after the pass: %+v, %v", got, err)
	}
}

func TestRequestFullScanNeedsAScheduler(t *testing.T) {
	idx := newRestoreTestIndex(t, "rescan_off", nil, nil)
	if err := idx.RequestFullScan(); !errors.Is(err, ErrNoScheduler) {
		t.Errorf("err = %v", err)
	}
}
