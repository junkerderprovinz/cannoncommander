package sqldb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestEveryConnectionCommitsWithoutWaitingForTheDisk(t *testing.T) {
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "main.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ctx := context.Background()
	for range 3 {
		conn, err := store.DB().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Held open, so the next round gets a connection of its own.
		defer func(c *sql.Conn) { c.Close() }(conn)
		var mode int
		if err := conn.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if mode != 1 {
			t.Errorf("synchronous = %d, want 1 (NORMAL)", mode)
		}
	}
}
