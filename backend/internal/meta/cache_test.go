package meta

import (
	"context"
	"database/sql"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func TestOlderRowsWithWordsAreReadAgain(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var first, second []migrate.Migration
	for _, m := range migrate.Registered(migrate.Index) {
		switch m.Name {
		case "meta/001_cache":
			first = append(first, m)
		case "meta/002_words_as_codes":
			second = append(second, m)
		}
	}
	ctx := context.Background()
	if _, err := migrate.ApplyList(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	rows := map[string]group{
		"/mnt/user/a.jpg":  gImage | gSums,
		"/mnt/user/b.mkv":  gAV | gTags,
		"/mnt/user/c.zip":  gArchive,
		"/mnt/user/d.exe":  gProgram,
		"/mnt/user/e.pdf":  gPaged | gOffice,
		"/mnt/user/f.txt":  gText | gSums,
		"/mnt/user/g.iso":  gDisk,
		"/mnt/user/h.flac": gTags,
	}
	for p, done := range rows {
		if _, err := db.Exec(`INSERT INTO meta_cache (path, size, mtime, done, orientation) VALUES (?, 1, 1, ?, 'Querformat')`, p, done); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrate.ApplyList(ctx, db, append(first, second...)); err != nil {
		t.Fatal(err)
	}

	got, err := db.Query(`SELECT path FROM meta_cache`)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	var kept []string
	for got.Next() {
		var p string
		if err := got.Scan(&p); err != nil {
			t.Fatal(err)
		}
		kept = append(kept, path.Base(p))
	}
	slices.Sort(kept)
	if want := "e.pdf f.txt g.iso h.flac"; strings.Join(kept, " ") != want {
		t.Errorf("kept %v, want %s", kept, want)
	}
}
