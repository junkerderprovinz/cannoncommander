package migrate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	_ "modernc.org/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestApplyRunsEachMigrationOnce(t *testing.T) {
	db := openDB(t)
	ms := []Migration{
		SQL("jobs/001_create", `CREATE TABLE jobs (id INTEGER PRIMARY KEY)`),
		SQL("trash/001_create", `CREATE TABLE trash_items (id INTEGER PRIMARY KEY)`),
	}
	ctx := context.Background()

	applied, err := ApplyList(ctx, db, ms)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{"jobs/001_create", "trash/001_create"}) {
		t.Fatalf("applied = %v", applied)
	}

	ms = append(ms, SQL("jobs/002_settings", `CREATE TABLE job_settings (k TEXT)`))
	applied, err = ApplyList(ctx, db, ms)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{"jobs/002_settings"}) {
		t.Fatalf("second run applied %v, want only the new one", applied)
	}
}

func TestFailedMigrationLeavesNoTrace(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	boom := errors.New("boom")
	ms := []Migration{{Name: "labels/001_create", Up: func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE label_tags (id INTEGER)`); err != nil {
			return err
		}
		return boom
	}}}

	if _, err := ApplyList(ctx, db, ms); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'label_tags'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("table from the failed migration is still there")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM cc_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("failed migration was recorded")
	}
}

func TestRegisteredSortsByName(t *testing.T) {
	const target Target = "test-sort"
	Register(target, SQL("undo/001_create", "SELECT 1"), SQL("jobs/002_b", "SELECT 1"), SQL("jobs/001_a", "SELECT 1"))

	var names []string
	for _, m := range Registered(target) {
		names = append(names, m.Name)
	}
	if !slices.Equal(names, []string{"jobs/001_a", "jobs/002_b", "undo/001_create"}) {
		t.Fatalf("names = %v", names)
	}
}

func TestRegisterRejectsDuplicatesAndBadNames(t *testing.T) {
	const target Target = "test-dup"
	Register(target, SQL("prefs/001_create", "SELECT 1"))
	for _, name := range []string{"prefs/001_create", "noslash", "/x", "x/"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Register(%q) did not panic", name)
				}
			}()
			Register(target, SQL(name, "SELECT 1"))
		}()
	}
}
