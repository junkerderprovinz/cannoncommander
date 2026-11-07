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
	// No folder sizes are stored yet, so the start runs a pass of its own.
	waitForPasses(t, idx, 1)
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

func TestThePlanNamesTheNextScanAndItsInterval(t *testing.T) {
	idx := newRestoreTestIndex(t, "plan", []string{"a"}, persistedScanners("/", "/a/"))
	idx.setupMultiScanner(false)
	waitForPasses(t, idx, 1)
	deadline := time.Now().Add(10 * time.Second)
	for idx.NextScan().IsZero() {
		if time.Now().After(deadline) {
			t.Fatal("no scan planned after the first pass")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if next := idx.NextScan(); !next.After(time.Now().Add(-time.Second)) {
		t.Errorf("next scan %v lies in the past", next)
	}
	every := idx.ScanEvery()
	if every < scanScheduleTiers[0] || every > scanScheduleTiers[len(scanScheduleTiers)-1] {
		t.Errorf("every = %v", every)
	}
}

func TestAScanBreaksOffWhenTheSchedulerStops(t *testing.T) {
	idx := newRestoreTestIndex(t, "stopped", []string{"a/tief"}, persistedScanners("/", "/a/"))
	idx.setupMultiScannerEx(false, false)
	if err := os.WriteFile(filepath.Join(idx.Path, "a", "tief", "neu.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx.stopScheduler()
	s := idx.scanners["/a/"]
	var last time.Time
	s.withStatsRLock(func() { last = s.lastScanned })
	s.executeScan()
	if got, _ := idx.db.GetItem("stopped", "/a/tief/neu.txt"); got != nil {
		t.Error("the scan went on after the stop")
	}
	s.withStatsRLock(func() {
		if !s.lastScanned.Equal(last) || s.fullScanCounter != 0 {
			t.Errorf("the broken off scan counts as done: last %v, counter %d", s.lastScanned, s.fullScanCounter)
		}
	})
	if metaStore.(*fakeMetaStore).saved != nil {
		t.Error("half a scan was saved")
	}
}
