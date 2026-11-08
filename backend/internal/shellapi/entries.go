package shellapi

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
)

func init() {
	migrate.Register(migrate.Main, migrate.SQL("shellapi/001_menu_entries",
		`CREATE TABLE shell_menu_entries (
			key TEXT PRIMARY KEY,
			label TEXT NOT NULL,
			icon TEXT NOT NULL,
			hidden INTEGER NOT NULL DEFAULT 0,
			seen_at INTEGER NOT NULL
		)`,
	))
}

// entries are the rows other programs put into the system's menu, as the
// app saw them, and which of them the person hid. The desktop app has one
// person, so they are not kept per user.
type entries struct{ db *sql.DB }

type entryJSON struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Icon   string `json:"icon,omitempty"`
	Hidden bool   `json:"hidden"`
}

// remember keeps the rows a menu just held, with their latest label and
// picture.
func (s entries) remember(ctx context.Context, seen []nativeshell.Entry) error {
	if len(seen) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	at := time.Now().Unix()
	for _, e := range seen {
		_, err := tx.ExecContext(ctx, `INSERT INTO shell_menu_entries (key, label, icon, seen_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET label = excluded.label, icon = excluded.icon, seen_at = excluded.seen_at`,
			e.Key, e.Label, e.Icon, at)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s entries) hidden(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key FROM shell_menu_entries WHERE hidden = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

func (s entries) list(ctx context.Context) ([]entryJSON, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, label, icon, hidden FROM shell_menu_entries ORDER BY label COLLATE NOCASE, key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entryJSON{}
	for rows.Next() {
		var e entryJSON
		if err := rows.Scan(&e.Key, &e.Label, &e.Icon, &e.Hidden); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// hide hides the rows named by keys and shows every other one again.
func (s entries) hide(ctx context.Context, keys []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE shell_menu_entries SET hidden = 0`); err != nil {
		return err
	}
	if len(keys) > 0 {
		args := make([]any, len(keys))
		for i, k := range keys {
			args[i] = k
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
		if _, err := tx.ExecContext(ctx, `UPDATE shell_menu_entries SET hidden = 1 WHERE key IN (`+marks+`)`, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
