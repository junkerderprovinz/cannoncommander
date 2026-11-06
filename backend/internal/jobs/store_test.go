package jobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// olderDB is a main database as the first version of the jobs table left
// it, holding jobs written back then.
func olderDB(t *testing.T, jobs ...[3]string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "main.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var first []migrate.Migration
	for _, m := range migrate.Registered(migrate.Main) {
		if m.Name == "jobs/001_create" {
			first = append(first, m)
		}
	}
	if _, err := migrate.ApplyList(context.Background(), db, first); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UnixMilli()
	for i, j := range jobs {
		id := j[0]
		if _, err := db.Exec(`INSERT INTO jobs (id, kind, owner, state, position, name, dest, params, answers, cap,
			wait_parity, done_bytes, done_files, current, error, undo, result, created_at, started_at, finished_at)
			VALUES (?, ?, 'jdp', 'failed', ?, ?, '/mnt/user', '{}', '{}', 0, 0, 0, 0, '', 'Platte voll', '', '', ?, ?, ?)`,
			id, j[1], i, j[2], at, at, at); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO job_items (job_id, seq, path, name, ref, state, resumed, bytes, files, error)
			VALUES (?, 0, '/mnt/user/a', 'a', '', 'failed', 0, 0, 0, 'Platte voll')`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	return db
}

func openOn(t *testing.T, db *sql.DB) *Manager {
	t.Helper()
	m, err := New(v1.Deps{DB: db, Hub: hub.New(hub.Options{})}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func TestOlderJobsKeepTheirErrorsWithoutACode(t *testing.T) {
	m := openOn(t, olderDB(t, [3]string{"a", "copy", "Urlaub"}))
	got, ok := m.Get("a", true)
	if !ok {
		t.Fatal("the older job is gone")
	}
	if got.Error != "Platte voll" || got.ErrorCode != "" || got.ErrorParams != nil {
		t.Errorf("job error %q %q %v", got.Error, got.ErrorCode, got.ErrorParams)
	}
	if it := got.Items[0]; it.Error != "Platte voll" || it.ErrorCode != "" || it.ErrorParams != nil {
		t.Errorf("item error %q %q %v", it.Error, it.ErrorCode, it.ErrorParams)
	}
}

func TestOlderJobNamesReadLikeNewOnes(t *testing.T) {
	m := openOn(t, olderDB(t,
		[3]string{"text", "contentSearch", "„plex server“"},
		[3]string{"all", "reindex", "Alle Quellen"},
		[3]string{"one", "reindex", "Disk 1"},
		[3]string{"quoted", "copy", "„Urlaub“"},
	))
	want := map[string]string{"text": "plex server", "all": "All sources", "one": "Disk 1", "quoted": "„Urlaub“"}
	for id, name := range want {
		if got, _ := m.Get(id, false); got.Name != name {
			t.Errorf("%s is named %q, want %q", id, got.Name, name)
		}
	}
}
