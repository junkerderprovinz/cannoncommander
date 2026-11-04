package sqldb

import (
	"path/filepath"
	"testing"
)

func tableExists(t *testing.T, s *SQLStore, name string) bool {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func TestTheOldShareLinksLeaveTheDatabase(t *testing.T) {
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if tableExists(t, store, "shares") {
		t.Fatal("a new database has the old share table")
	}

	for _, q := range []string{
		`CREATE TABLE shares (hash TEXT PRIMARY KEY, user_id TEXT NOT NULL, share_settings TEXT NOT NULL)`,
		`INSERT INTO shares VALUES ('abc', '1', '{}')`,
		`INSERT INTO settings (key, value) VALUES ('shareDefaults.default', '{}'), ('userDefaults.default', '{}')`,
		`UPDATE schema_version SET version = 4`,
	} {
		if _, err := store.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := runMigrations(store.db, 4); err != nil {
		t.Fatal(err)
	}
	if tableExists(t, store, "shares") {
		t.Fatal("the old share table is still there")
	}
	if _, err := store.GetSetting("shareDefaults.default"); err == nil {
		t.Fatal("the old share defaults are still there")
	}
	if _, err := store.GetSetting("userDefaults.default"); err != nil {
		t.Fatalf("other settings must stay: %v", err)
	}
}

func TestTheQuotaTablesLeaveTheDatabase(t *testing.T) {
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, q := range []string{
		`CREATE TABLE quotas (id TEXT PRIMARY KEY NOT NULL)`,
		`CREATE TABLE quota_counters (quota_id TEXT PRIMARY KEY NOT NULL)`,
		`UPDATE schema_version SET version = 5`,
	} {
		if _, err := store.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := runMigrations(store.db, 5); err != nil {
		t.Fatal(err)
	}
	if tableExists(t, store, "quotas") || tableExists(t, store, "quota_counters") {
		t.Fatal("the quota tables are still there")
	}
}
