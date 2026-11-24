package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dbsql "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sql"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// typedTree writes /pieces/ with dirs folders of perDir files ending in
// ext, each a kilobyte of binary data like a storj piece.
func typedTree(b testing.TB, dirs, perDir int, ext string) string {
	b.Helper()
	root := b.TempDir()
	body := make([]byte, 1024)
	for i := range body {
		body[i] = byte(i)
	}
	for d := range dirs {
		dir := filepath.Join(root, "pieces", fmt.Sprintf("%02x", d%256), fmt.Sprintf("%04d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for f := range perDir {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("piece-%04d%s", f, ext)), body, 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root
}

func scanIndex(b testing.TB, root string) *Index {
	b.Helper()
	old := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = b.TempDir()
	db, _, err := dbsql.NewIndexDB("scanbench", "OFF", 1000, 32, true)
	settings.Config.Server.CacheDir = old
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	return &Index{
		Source:              settings.Source{Name: "scanbench", Path: root, Config: settings.SourceConfig{UseLogicalSize: true}},
		db:                  db,
		mock:                true,
		folderSizes:         map[string]uint64{},
		folderSizesUnsynced: map[string]struct{}{},
		scanUpdatedPaths:    map[string]bool{},
	}
}

// BenchmarkRepeatFullScan reads a tree that the index already holds, the
// way every full pass after the first does, and reports files per second.
// Files with an extension the type detection does not know need their
// header read unless the index already holds their type.
func BenchmarkRepeatFullScan(b *testing.B) {
	const dirs, perDir = 1024, 64
	for _, ext := range []string{".sj1", ".jpg"} {
		b.Run("ext="+ext, func(b *testing.B) {
			idx := scanIndex(b, typedTree(b, dirs, perDir, ext))
			if err := idx.createScanner("/pieces/").runFullScanChild(); err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				if err := idx.createScanner("/pieces/").runFullScanChild(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(dirs*perDir)*float64(b.N)/b.Elapsed().Seconds(), "files/s")
		})
	}
}
