package meta

import (
	"context"
	"database/sql"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

// The cache lives in the index database: it is as rebuildable as the
// index itself and goes with it when the index is thrown away.
func init() {
	migrate.Register(migrate.Index, migrate.SQL("meta/001_cache",
		`CREATE TABLE IF NOT EXISTS meta_cache (
			path TEXT PRIMARY KEY,
			size INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			done INTEGER NOT NULL,
			width INTEGER, height INTEGER,
			camera TEXT, make TEXT, lens TEXT, focal TEXT, aperture TEXT, exposure TEXT, iso INTEGER, flash TEXT,
			taken INTEGER, lat REAL, lon REAL, orientation TEXT, depth TEXT, dpi TEXT,
			duration INTEGER, vcodec TEXT, acodec TEXT, framerate TEXT, vbitrate INTEGER, aspect TEXT, hdr TEXT,
			audiotracks TEXT, subtitles TEXT, abitrate TEXT, samplerate TEXT, channels TEXT,
			artist TEXT, album TEXT, title TEXT, albumartist TEXT, track INTEGER, year INTEGER, genre TEXT, composer TEXT,
			pages INTEGER, author TEXT, lastauthor TEXT, doctitle TEXT, subject TEXT, app TEXT, encrypted INTEGER,
			words INTEGER, lines INTEGER, encoding TEXT,
			unpacked INTEGER, entries INTEGER, method TEXT, archcrypt INTEGER,
			fs TEXT, volume TEXT, bootable INTEGER,
			version TEXT, vendor TEXT, arch TEXT, signed INTEGER,
			sha256 TEXT, crc32 TEXT, md5 TEXT, sha1 TEXT
		)`,
	))
}

// fieldNames is the order of the field columns in every query.
var fieldNames = func() []string {
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}()

// entry is one cached file. It is only valid while the file keeps the size
// and the modification time it had when it was read.
type entry struct {
	size  int64
	mtime int64
	done  group
	rec   Record
}

// cache is meta_cache in a database that may come and go: the index
// database is opened after the routes are set up and can be rebuilt.
type cache struct {
	db func() *sql.DB
}

func (c cache) handle() *sql.DB {
	if c.db == nil {
		return nil
	}
	return c.db()
}

func quoted(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = `"` + n + `"`
	}
	return strings.Join(out, ", ")
}

// load reads the cached entries of paths. Paths without one are missing.
func (c cache) load(ctx context.Context, paths []string) (map[string]entry, error) {
	db := c.handle()
	out := map[string]entry{}
	if db == nil || len(paths) == 0 {
		return out, nil
	}
	cols := "path, size, mtime, done, " + quoted(fieldNames)
	for chunk := range slices.Chunk(paths, 400) {
		args := make([]any, len(chunk))
		for i, p := range chunk {
			args[i] = p
		}
		rows, err := db.QueryContext(ctx, `SELECT `+cols+` FROM meta_cache WHERE path IN (?`+strings.Repeat(",?", len(chunk)-1)+`)`, args...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			p, e, err := scanEntry(rows)
			if err != nil {
				rows.Close()
				return out, err
			}
			out[p] = e
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}
	return out, nil
}

func scanEntry(rows *sql.Rows) (string, entry, error) {
	var p string
	var e entry
	vals := make([]any, len(fieldNames))
	dest := []any{&p, &e.size, &e.mtime, &e.done}
	for i := range vals {
		dest = append(dest, &vals[i])
	}
	if err := rows.Scan(dest...); err != nil {
		return "", e, err
	}
	e.rec = Record{}
	for i, n := range fieldNames {
		if v := typed(fields[n], vals[i]); v != nil {
			e.rec[n] = v
		}
	}
	return p, e, nil
}

// typed turns what the driver hands back into the Go type of the field.
func typed(k kind, v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case int64:
		switch k {
		case boolField:
			return x != 0
		case realField:
			return float64(x)
		case textField:
			return nil
		}
		return x
	case float64:
		if k == realField {
			return x
		}
		if k == intField {
			return int64(x)
		}
	case string:
		if k == textField {
			return x
		}
	case []byte:
		if k == textField {
			return string(x)
		}
	}
	return nil
}

func (c cache) save(ctx context.Context, p string, e entry) error {
	db := c.handle()
	if db == nil {
		return nil
	}
	args := []any{p, e.size, e.mtime, int64(e.done)}
	for _, n := range fieldNames {
		v := e.rec[n]
		if b, ok := v.(bool); ok {
			v = 0
			if b {
				v = 1
			}
		}
		args = append(args, v)
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	_, err := db.ExecContext(ctx, `INSERT OR REPLACE INTO meta_cache (path, size, mtime, done, `+quoted(fieldNames)+`) VALUES (`+marks+`)`, args...)
	return err
}

// forget drops the entries below prefix whose file is gone, as told by
// exists. It returns how many went.
func (c cache) forget(ctx context.Context, prefix string, exists func(string) bool) (int, error) {
	db := c.handle()
	if db == nil {
		return 0, nil
	}
	lo := strings.TrimSuffix(prefix, "/") + "/"
	hi := lo[:len(lo)-1] + "0"
	rows, err := db.QueryContext(ctx, `SELECT path FROM meta_cache WHERE path > ? AND path < ?`, lo, hi)
	if err != nil {
		return 0, err
	}
	var gone []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return 0, err
		}
		if !exists(p) {
			gone = append(gone, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range gone {
		if _, err := db.ExecContext(ctx, `DELETE FROM meta_cache WHERE path = ?`, p); err != nil {
			return 0, err
		}
	}
	return len(gone), nil
}

// count returns how many files below prefix have cached metadata.
func (c cache) count(ctx context.Context, prefix string) (int64, error) {
	db := c.handle()
	if db == nil {
		return 0, nil
	}
	lo := strings.TrimSuffix(prefix, "/") + "/"
	hi := lo[:len(lo)-1] + "0"
	var n int64
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM meta_cache WHERE path > ? AND path < ?`, lo, hi).Scan(&n)
	return n, err
}
