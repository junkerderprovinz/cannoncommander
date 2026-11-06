package sql

import (
	"path"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqlitebusy"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
)

// The index keeps inode, link count, owner and mode next to each entry, so
// hard links and owners can be searched without walking the disks. The
// birth time already lives in created_time.
func init() {
	migrate.Register(migrate.Index, migrate.SQL("index/001_unix_attributes",
		`ALTER TABLE index_items ADD COLUMN ino INTEGER`,
		`ALTER TABLE index_items ADD COLUMN nlink INTEGER`,
		`ALTER TABLE index_items ADD COLUMN uid INTEGER`,
		`ALTER TABLE index_items ADD COLUMN gid INTEGER`,
		`ALTER TABLE index_items ADD COLUMN mode INTEGER`,
		`CREATE INDEX IF NOT EXISTS idx_source_ino ON index_items(source, ino)`,
	))
}

// upsertItem writes one entry. Inode details arrive as NULL where the
// platform has none, and then the row keeps what it had.
const upsertItem = `
	INSERT INTO index_items (source, path, parent_path, name, size, created_time, mod_time, type, is_dir, is_hidden, has_preview, last_updated, ino, nlink, uid, gid, mode)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(source, path) DO UPDATE SET
		parent_path = excluded.parent_path,
		name = excluded.name,
		size = excluded.size,
		created_time = excluded.created_time,
		mod_time = excluded.mod_time,
		type = excluded.type,
		is_dir = excluded.is_dir,
		is_hidden = excluded.is_hidden,
		has_preview = excluded.has_preview,
		last_updated = excluded.last_updated,
		ino = COALESCE(excluded.ino, index_items.ino),
		nlink = COALESCE(excluded.nlink, index_items.nlink),
		uid = COALESCE(excluded.uid, index_items.uid),
		gid = COALESCE(excluded.gid, index_items.gid),
		mode = COALESCE(excluded.mode, index_items.mode)
	`

func known(a iteminfo.UnixAttrs, v int64) any {
	if !a.Known {
		return nil
	}
	return v
}

func inode(a iteminfo.UnixAttrs) any { return known(a, int64(a.Inode)) }
func nlink(a iteminfo.UnixAttrs) any { return known(a, int64(a.Nlink)) }
func uid(a iteminfo.UnixAttrs) any   { return known(a, int64(a.UID)) }
func gid(a iteminfo.UnixAttrs) any   { return known(a, int64(a.GID)) }
func mode(a iteminfo.UnixAttrs) any  { return known(a, int64(a.Mode)) }

// FolderCounts returns how many entries the index holds directly inside each
// of dirs, which are folder index paths with a trailing slash. Folders the
// index has never seen are missing from the result.
func (db *IndexDB) FolderCounts(source string, dirs []string) (map[string]int, error) {
	out := make(map[string]int, len(dirs))
	err := inChunks(dirs, func(marks string, args []any) error {
		args = append([]any{source}, args...)
		rows, err := db.Query(`SELECT path FROM index_items WHERE source = ? AND is_dir = 1 AND path IN (`+marks+`)`, args...)
		if err != nil {
			return sqlitebusy.Wrap(err)
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return err
			}
			out[p] = 0
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = db.Query(`SELECT parent_path, COUNT(*) FROM index_items WHERE source = ? AND parent_path IN (`+marks+`) GROUP BY parent_path`, args...)
		if err != nil {
			return sqlitebusy.Wrap(err)
		}
		defer rows.Close()
		for rows.Next() {
			var p string
			var n int
			if err := rows.Scan(&p, &n); err != nil {
				return err
			}
			out[p] = n
		}
		return rows.Err()
	})
	return out, err
}

// FilesNamed returns, per folder in dirs, the files directly inside whose
// name is one of names in any case. Names must be given in lower case.
func (db *IndexDB) FilesNamed(source string, dirs, names []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(names) == 0 {
		return out, nil
	}
	nameMarks := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")
	err := inChunks(dirs, func(marks string, args []any) error {
		all := append([]any{source}, args...)
		for _, n := range names {
			all = append(all, n)
		}
		rows, err := db.Query(`SELECT parent_path, name FROM index_items
			WHERE source = ? AND is_dir = 0 AND parent_path IN (`+marks+`) AND lower(name) IN (`+nameMarks+`)`, all...)
		if err != nil {
			return sqlitebusy.Wrap(err)
		}
		defer rows.Close()
		for rows.Next() {
			var dir, name string
			if err := rows.Scan(&dir, &name); err != nil {
				return err
			}
			out[dir] = append(out[dir], name)
		}
		return rows.Err()
	})
	return out, err
}

// TreeCounts counts the files and folders anywhere below dir, a folder index
// path with a trailing slash.
func (db *IndexDB) TreeCounts(source, dir string) (files, dirs int64, err error) {
	err = db.QueryRow(`SELECT COALESCE(SUM(is_dir = 0), 0), COALESCE(SUM(is_dir = 1), 0)
		FROM index_items WHERE source = ? AND path > ? AND path < ?`,
		source, dir, getNextPathPrefix(dir)).Scan(&files, &dirs)
	return files, dirs, sqlitebusy.Wrap(err)
}

// inChunks keeps IN lists well below SQLite's limit on bound parameters.
func inChunks(keys []string, fn func(marks string, args []any) error) error {
	const size = 500
	for start := 0; start < len(keys); start += size {
		chunk := keys[start:min(start+size, len(keys))]
		args := make([]any, len(chunk))
		for i, k := range chunk {
			args[i] = k
		}
		if err := fn(strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ","), args); err != nil {
			return err
		}
	}
	return nil
}

// PathsByInode returns the files of source that carry the inode ino.
func (db *IndexDB) PathsByInode(source string, ino uint64) ([]string, error) {
	rows, err := db.Query(`SELECT path FROM index_items WHERE source = ? AND ino = ? AND is_dir = 0`, source, int64(ino))
	if err != nil {
		return nil, sqlitebusy.Wrap(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Children returns the entries the index holds directly inside dir, a folder
// index path with a trailing slash, and whether each is a folder.
func (db *IndexDB) Children(source, dir string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT path, is_dir FROM index_items WHERE source = ? AND parent_path = ?`, source, dir)
	if err != nil {
		return nil, sqlitebusy.Wrap(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		var isDir bool
		if err := rows.Scan(&p, &isDir); err != nil {
			return nil, err
		}
		out[p] = isDir
	}
	return out, rows.Err()
}

// treeBatch is how many rows one statement of DeleteTrees or MoveTree
// changes, so a large folder never holds the database for long.
const treeBatch = 1000

// DeleteTrees removes the rows at paths and, for folders, everything below.
// Folder paths carry their trailing slash.
func (db *IndexDB) DeleteTrees(source string, paths []string) error {
	err := inChunks(paths, func(marks string, args []any) error {
		_, err := db.Exec(`DELETE FROM index_items WHERE source = ? AND path IN (`+marks+`)`, append([]any{source}, args...)...)
		return sqlitebusy.Wrap(err)
	})
	if err != nil {
		return err
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, "/") {
			continue
		}
		if err := db.inBatches(`DELETE FROM index_items WHERE rowid IN (
			SELECT rowid FROM index_items WHERE source = ? AND path > ? AND path < ? LIMIT ?)`,
			source, p, getNextPathPrefix(p), treeBatch); err != nil {
			return err
		}
	}
	return nil
}

// MoveTree gives the row at from, and for a folder every row below it, the
// path it has under to. Rows already at to are replaced. It reports whether
// the index held from at all.
func (db *IndexDB) MoveTree(source, from, to string) (bool, error) {
	if err := db.DeleteTrees(source, []string{to}); err != nil {
		return false, err
	}
	res, err := db.Exec(`UPDATE OR REPLACE index_items SET path = ?, parent_path = ?, name = ?, last_updated = ?
		WHERE source = ? AND path = ?`,
		to, getParentPath(to), path.Base(strings.TrimSuffix(to, "/")), time.Now().Unix(), source, from)
	if err != nil {
		return false, sqlitebusy.Wrap(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	if !strings.HasSuffix(from, "/") {
		return true, nil
	}
	// Moved rows leave the range, so each batch picks up the next ones.
	return true, db.inBatches(`UPDATE OR REPLACE index_items
		SET path = ? || substr(path, length(?) + 1), parent_path = ? || substr(parent_path, length(?) + 1), last_updated = ?
		WHERE rowid IN (SELECT rowid FROM index_items WHERE source = ? AND path > ? AND path < ? LIMIT ?)`,
		to, from, to, from, time.Now().Unix(), source, from, getNextPathPrefix(from), treeBatch)
}

// inBatches runs a statement that changes at most treeBatch rows until it
// changes fewer.
func (db *IndexDB) inBatches(query string, args ...any) error {
	for {
		res, err := db.Exec(query, args...)
		if err != nil {
			return sqlitebusy.Wrap(err)
		}
		if n, _ := res.RowsAffected(); n < treeBatch {
			return nil
		}
	}
}
