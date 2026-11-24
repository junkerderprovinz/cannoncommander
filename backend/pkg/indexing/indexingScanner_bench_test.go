package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"sync"
	"testing"
	"time"

	dbsql "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sql"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// benchTrees holds one synthetic tree per size for the whole test binary,
// since writing it takes far longer than indexing it. The trees lie in the
// cache directory TestMain removes.
var benchTrees sync.Map

// benchTree returns a source root holding /cloud/ with dirs folders below it
// and three small files in each. Folders fan out eight ways, so the deepest
// index paths look like /cloud/folder-0001/.../folder-0007/, about as long as
// the ones in large Nextcloud and OpenCloud trees.
func benchTree(b testing.TB, dirs int) string {
	if root, ok := benchTrees.Load(dirs); ok {
		return root.(string)
	}
	root, err := os.MkdirTemp(settings.Config.Server.CacheDir, fmt.Sprintf("tree-%d-", dirs))
	if err != nil {
		b.Fatal(err)
	}
	queue := []string{filepath.Join(root, "cloud")}
	if err := os.Mkdir(queue[0], 0o755); err != nil {
		b.Fatal(err)
	}
	for made := 0; made < dirs; {
		parent := queue[0]
		queue = queue[1:]
		for i := 0; i < 8 && made < dirs; i++ {
			dir := filepath.Join(parent, fmt.Sprintf("folder-%04d", i))
			if err := os.Mkdir(dir, 0o755); err != nil {
				b.Fatal(err)
			}
			for f := 0; f < 3; f++ {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("picture-%02d.jpg", f)), []byte("1234"), 0o644); err != nil {
					b.Fatal(err)
				}
			}
			queue = append(queue, dir)
			made++
		}
	}
	benchTrees.Store(dirs, root)
	return root
}

func benchIndex(b testing.TB, root string) *Index {
	b.Helper()
	old := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = b.TempDir()
	db, _, err := dbsql.NewIndexDB("bench", "OFF", 1000, 32, true)
	settings.Config.Server.CacheDir = old
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	return &Index{
		Source:           settings.Source{Name: "bench", Path: root, Config: settings.SourceConfig{UseLogicalSize: true}},
		db:               db,
		mock:             true,
		folderSizes:      newFolderSizes(),
		scanUpdatedPaths: map[string]bool{},
	}
}

// heapSampler records the largest heap seen while a scan runs.
type heapSampler struct {
	stop chan struct{}
	done chan uint64
}

func sampleHeap() heapSampler {
	h := heapSampler{stop: make(chan struct{}), done: make(chan uint64)}
	go func() {
		s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		var peak uint64
		t := time.NewTicker(time.Millisecond)
		defer t.Stop()
		for {
			metrics.Read(s)
			peak = max(peak, s[0].Value.Uint64())
			select {
			case <-h.stop:
				h.done <- peak
				return
			case <-t.C:
			}
		}
	}()
	return h
}

func (h heapSampler) peak() uint64 {
	close(h.stop)
	return <-h.done
}

// liveHeap collects garbage first, so what it reports is what is still held.
func liveHeap() uint64 {
	runtime.GC()
	s := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

func perDir(v uint64, base uint64, dirs int) float64 {
	if v < base {
		return 0
	}
	return float64(v-base) / float64(dirs)
}

// BenchmarkFullScan reads a whole tree the way a child scanner's full pass
// does and reports, per folder, what the index keeps in memory afterwards
// (live) and the most the heap held during the pass (peak).
func BenchmarkFullScan(b *testing.B) {
	for _, dirs := range []int{32768} {
		b.Run(fmt.Sprintf("dirs=%d", dirs), func(b *testing.B) {
			root := benchTree(b, dirs)
			var live, peak float64
			for b.Loop() {
				b.StopTimer()
				idx := benchIndex(b, root)
				s := idx.createScanner("/cloud/")
				base := liveHeap()
				sampler := sampleHeap()
				b.StartTimer()
				if err := s.runFullScanChild(); err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				p := sampler.peak()
				live, peak = perDir(liveHeap(), base, dirs), perDir(p, base, dirs)
				if n := idx.folderSizes.len(); n < dirs {
					b.Fatalf("index holds %d folders, want at least %d", n, dirs)
				}
				runtime.KeepAlive(idx)
				b.StartTimer()
			}
			b.ReportMetric(live, "live-B/dir")
			b.ReportMetric(peak, "peak-B/dir")
		})
	}
}

// BenchmarkRestoreFolderSizes loads the folder sizes a previous pass stored,
// which is what every start of the app does before the first scan.
func BenchmarkRestoreFolderSizes(b *testing.B) {
	for _, dirs := range []int{32768} {
		b.Run(fmt.Sprintf("dirs=%d", dirs), func(b *testing.B) {
			root := benchTree(b, dirs)
			first := benchIndex(b, root)
			if err := first.createScanner("/cloud/").runFullScanChild(); err != nil {
				b.Fatal(err)
			}
			var live float64
			for b.Loop() {
				b.StopTimer()
				idx := &Index{Source: first.Source, db: first.db, mock: true, folderSizes: newFolderSizes()}
				base := liveHeap()
				b.StartTimer()
				if !idx.restoreFolderSizes() {
					b.Fatal("nothing restored")
				}
				b.StopTimer()
				live = perDir(liveHeap(), base, dirs)
				runtime.KeepAlive(idx)
				b.StartTimer()
			}
			b.ReportMetric(live, "live-B/dir")
		})
	}
}
