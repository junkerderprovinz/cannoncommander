package remotes

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func init() {
	migrate.Register(migrate.Main,
		// rclone.conf holds the type and the parameters, this table what
		// rclone does not know: where a remote is mounted and whether it
		// should be.
		migrate.SQL("remotes/001_create",
			`CREATE TABLE remotes (
				name TEXT PRIMARY KEY,
				type TEXT NOT NULL,
				path TEXT NOT NULL DEFAULT '',
				mount TEXT NOT NULL UNIQUE,
				mount_on_start INTEGER NOT NULL DEFAULT 1,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
		),
		// Mount folders lie below /remotes, away from the host's /mnt. A row
		// outside it gets /remotes and its name, which no other row holds:
		// names are unique, and when this runs every row lies below /mnt.
		migrate.SQL("remotes/002_outside_mnt",
			`UPDATE remotes SET mount = '/remotes/' || name, updated_at = unixepoch() WHERE mount NOT LIKE '/remotes/%'`,
		),
	)
}

// row is a remote as the database keeps it.
type row struct {
	Name         string
	Type         string
	Path         string
	Mount        string
	MountOnStart bool
}

func (r row) fs() string { return r.Name + ":" + r.Path }

type store struct{ db *sql.DB }

func (s store) all(ctx context.Context) ([]row, error) {
	rs, err := s.db.QueryContext(ctx, `SELECT name, type, path, mount, mount_on_start FROM remotes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.Name, &r.Type, &r.Path, &r.Mount, &r.MountOnStart); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func (s store) get(ctx context.Context, name string) (row, bool, error) {
	var r row
	err := s.db.QueryRowContext(ctx, `SELECT name, type, path, mount, mount_on_start FROM remotes WHERE name = ?`, name).
		Scan(&r.Name, &r.Type, &r.Path, &r.Mount, &r.MountOnStart)
	if errors.Is(err, sql.ErrNoRows) {
		return row{}, false, nil
	}
	if err != nil {
		return row{}, false, err
	}
	return r, true, nil
}

func (s store) insert(ctx context.Context, r row) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `INSERT INTO remotes (name, type, path, mount, mount_on_start, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, r.Name, r.Type, r.Path, r.Mount, r.MountOnStart, now, now)
	return err
}

// replace overwrites the remote called old, which may get a new name.
func (s store) replace(ctx context.Context, old string, r row) error {
	_, err := s.db.ExecContext(ctx, `UPDATE remotes SET name = ?, type = ?, path = ?, mount = ?, mount_on_start = ?, updated_at = ?
		WHERE name = ?`, r.Name, r.Type, r.Path, r.Mount, r.MountOnStart, time.Now().Unix(), old)
	return err
}

func (s store) setMountOnStart(ctx context.Context, name string, on bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE remotes SET mount_on_start = ?, updated_at = ? WHERE name = ?`, on, time.Now().Unix(), name)
	return err
}

func (s store) remove(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM remotes WHERE name = ?`, name)
	return err
}
