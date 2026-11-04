package shares

import (
	"context"
	"crypto/rand"
	"database/sql"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func init() {
	migrate.Register(migrate.Main,
		migrate.SQL("shares/001_links",
			// The table called shares belongs to the links of the FileBrowser
			// code, so these get tables of their own.
			`CREATE TABLE share_links (
				id TEXT PRIMARY KEY,
				slug TEXT NOT NULL UNIQUE,
				owner_id INTEGER NOT NULL,
				owner TEXT NOT NULL,
				kind TEXT NOT NULL,
				name TEXT NOT NULL,
				password_hash TEXT NOT NULL,
				expires_at INTEGER,
				download_limit INTEGER NOT NULL,
				downloads INTEGER NOT NULL,
				views INTEGER NOT NULL,
				uploads INTEGER NOT NULL,
				upload_limit INTEGER NOT NULL,
				uploaded INTEGER NOT NULL,
				notify INTEGER NOT NULL,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE INDEX share_links_owner ON share_links (owner_id)`,
			`CREATE TABLE share_link_paths (
				share_id TEXT NOT NULL,
				seq INTEGER NOT NULL,
				path TEXT NOT NULL,
				PRIMARY KEY (share_id, seq)
			)`,
			`CREATE INDEX share_link_paths_path ON share_link_paths (path)`,
		),
		migrate.Migration{Name: "shares/002_secret", Up: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `CREATE TABLE share_secret (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				key BLOB NOT NULL
			)`); err != nil {
				return err
			}
			key := make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO share_secret (id, key) VALUES (1, ?)`, key)
			return err
		}},
		migrate.SQL("shares/003_message",
			`ALTER TABLE share_links ADD COLUMN message TEXT NOT NULL DEFAULT ''`,
		),
	)
}
