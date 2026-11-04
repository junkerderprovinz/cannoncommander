// Package fsindextest gives tests a real index database and fills it from a
// folder on disk, the way the scanner would.
package fsindextest

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	dbsql "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sql"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// New opens an empty index database with every registered index migration
// and makes it the one the indexing package hands out, until the test ends.
func New(t testing.TB) *dbsql.IndexDB {
	t.Helper()
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0o644, 0o755)
	}
	old := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = t.TempDir()
	tmp, err := dbsql.NewTempDB("index_test", &dbsql.TempDBConfig{})
	settings.Config.Server.CacheDir = old
	if err != nil {
		t.Fatal(err)
	}
	db := &dbsql.IndexDB{TempDB: tmp}
	if err := db.CreateIndexTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Apply(context.Background(), db.DB(), migrate.Index); err != nil {
		t.Fatal(err)
	}
	indexing.SetIndexDBForTesting(db)
	t.Cleanup(func() {
		indexing.SetIndexDBForTesting(nil)
		indexing.ClearTestIndices()
		db.Close()
	})
	return db
}

// Scan registers the source name with its root and writes everything below
// root into the index, folders with a trailing slash as the scanner does.
func Scan(t testing.TB, db *dbsql.IndexDB, source, root string) {
	t.Helper()
	indexing.SetTestIndex(source, root)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		key := "/" + filepath.ToSlash(rel)
		if rel == "." {
			key = "/"
		}
		fi := &iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{
			Name: d.Name(), Size: info.Size(), ModTime: info.ModTime(), Type: "text/plain",
			Hidden: strings.HasPrefix(d.Name(), "."),
		}}
		if d.IsDir() {
			fi.Type, fi.Size = "directory", 0
			if !strings.HasSuffix(key, "/") {
				key += "/"
			}
		}
		return db.InsertItem(source, key, fi)
	})
	if err != nil {
		t.Fatal(err)
	}
}
