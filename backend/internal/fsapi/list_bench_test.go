package fsapi

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkListOfAHugeFolder reads a folder of 50,000 files, the size at
// which a second stat per entry starts to show.
func BenchmarkListOfAHugeFolder(b *testing.B) {
	f := newFixture(b)
	dir := filepath.Join(f.share, "viele")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatal(err)
	}
	for i := range 50000 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("bild-%05d.jpg", i)), nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for b.Loop() {
		rec := f.get(b, "jdp", "/fs/list", "/mnt/user/viele")
		if rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}
