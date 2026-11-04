package prefs

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// named is a table of entries a user keeps under a name, each with one
// JSON column for the rest. Profiles and saved searches are stored this way.
type named struct {
	table, column string
	limit         int
}

type namedRow struct {
	ID   int64
	Name string
	Data []byte
}

// noEntry is a fresh problem each time, because WriteError fills in the
// instance of the problem it is given.
func noEntry() error { return v1.Errorf(http.StatusNotFound, "no such entry") }

func (n named) list(ctx context.Context, db *sql.DB, userID string) ([]namedRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, `+n.column+` FROM `+n.table+` WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []namedRow
	for rows.Next() {
		var row namedRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Data); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (n named) create(ctx context.Context, db *sql.DB, userID, name string, data any) (int64, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+n.table+` WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return 0, err
	}
	if count >= n.limit {
		return 0, v1.Errorf(http.StatusConflict, "there are too many entries already")
	}
	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, `INSERT INTO `+n.table+` (user_id, name, `+n.column+`, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		userID, name, string(raw), now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (n named) update(ctx context.Context, db *sql.DB, userID string, id int64, name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx, `UPDATE `+n.table+` SET name = ?, `+n.column+` = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		name, string(raw), time.Now().Unix(), id, userID)
	if err != nil {
		return err
	}
	return mustHit(res)
}

func (n named) delete(ctx context.Context, db *sql.DB, userID string, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM `+n.table+` WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return mustHit(res)
}

// mustHit turns a write that touched no row into a 404, which is also what
// another user's entry looks like.
func mustHit(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return noEntry()
	}
	return nil
}
