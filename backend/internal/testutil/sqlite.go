package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqlitebusy"
	_ "modernc.org/sqlite"
)

// MainDB opens a fresh main database in a temporary folder, set up like
// the real one, with every migration the test binary has registered.
func MainDB(t testing.TB) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "main.db") + "?mode=rwc&_journal_mode=WAL"
	db, err := sql.Open("sqlite", sqlitebusy.WithBusyTimeout(dsn))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	return db
}
