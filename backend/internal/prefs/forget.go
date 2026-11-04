package prefs

import (
	"context"
	"database/sql"
	"strconv"
)

// tables lists every table that keeps rows per user.
var tables = []string{"user_prefs", "device_prefs", "bookmarks", "rename_presets", "profiles", "saved_searches", "frequent_dirs"}

// Forget deletes everything kept for the user, for an account that is being
// deleted, so a new account can never inherit it.
func Forget(ctx context.Context, db *sql.DB, userID uint64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := strconv.FormatUint(userID, 10)
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE user_id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
