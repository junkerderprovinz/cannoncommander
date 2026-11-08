package fsindex

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqlitebusy"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// Entry is one file or folder below the folder a walk started in.
type Entry struct {
	// Path is the API path, Real the path on disk.
	Path string
	Real string
	Name string
	Dir  bool
	Size int64
	// MTime has whole seconds when it comes from the index. Created is
	// zero where the file system keeps no birth time.
	MTime   time.Time
	Created time.Time
	Hidden  bool
	// Mode holds the permission bits as chmod takes them, 0 when the
	// index does not know them.
	Mode uint32
}

// walkPage is how many index rows one query reads. Between two pages the
// index database is free for the scanner, which shares its connection.
const walkPage = 2000

// Walk calls fn for everything below the folder loc, a folder before what
// it holds. It reads the index when the index holds the folder, and the
// disk otherwise, and reports which one it was. When fn returns
// fs.SkipDir for a folder its contents are left out, any other error ends
// the walk.
func Walk(ctx context.Context, loc ccfs.Location, fn func(Entry) error) (indexed bool, err error) {
	if loc.Source == nil {
		return false, errors.New("walk needs a folder inside a source")
	}
	if db := indexing.GetIndexDB(); db != nil && indexing.Lookup(loc.Source.Name) != nil {
		known, err := walkIndex(ctx, db.DB(), loc, fn)
		if known || err != nil {
			return true, err
		}
	}
	return false, walkDisk(ctx, loc.Real, loc.Path, fn)
}

func walkIndex(ctx context.Context, db *sql.DB, loc ccfs.Location, fn func(Entry) error) (bool, error) {
	src := loc.Source
	dir := slashed(loc.Index)
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM index_items WHERE source = ? AND path = ?`, src.Name, dir).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, sqlitebusy.Wrap(err)
	}
	end := nextPrefix(dir)
	from, inclusive := dir, false
	for {
		op := ">"
		if inclusive {
			op = ">="
		}
		rows, err := db.QueryContext(ctx, `SELECT path, name, size, mod_time, created_time, is_dir, is_hidden, COALESCE(mode, 0)
			FROM index_items WHERE source = ? AND path `+op+` ? AND path < ? ORDER BY path LIMIT ?`,
			src.Name, from, end, walkPage)
		if err != nil {
			return true, sqlitebusy.Wrap(err)
		}
		var page []Entry
		var keys []string
		for rows.Next() {
			var e Entry
			var key string
			var mtime, created int64
			if err := rows.Scan(&key, &e.Name, &e.Size, &mtime, &created, &e.Dir, &e.Hidden, &e.Mode); err != nil {
				rows.Close()
				return true, err
			}
			e.MTime = time.Unix(mtime, 0)
			if created > 0 {
				e.Created = time.Unix(created, 0)
			}
			rel := strings.TrimSuffix(key, "/")
			e.Path = path.Join(src.Mount, rel)
			e.Real = filepath.Join(src.Root, filepath.FromSlash(rel))
			page = append(page, e)
			keys = append(keys, key)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return true, err
		}
		if len(page) == 0 {
			return true, nil
		}
		from, inclusive = keys[len(keys)-1], false
		for i, e := range page {
			if err := ctx.Err(); err != nil {
				return true, err
			}
			err := fn(e)
			if errors.Is(err, fs.SkipDir) && e.Dir {
				from, inclusive = nextPrefix(keys[i]), true
				break
			}
			if err != nil && !errors.Is(err, fs.SkipDir) {
				return true, err
			}
		}
		if len(page) < walkPage && !inclusive {
			return true, nil
		}
	}
}

// nextPrefix is the first key after every key that starts with p, for
// range scans over the primary key.
func nextPrefix(p string) string {
	r := []rune(p)
	r[len(r)-1]++
	return string(r)
}

// walkDisk reads the folders itself. A subfolder that cannot be read is
// passed over, the walk goes on with the rest.
func walkDisk(ctx context.Context, real, api string, fn func(Entry) error) error {
	dirents, err := os.ReadDir(real)
	if err != nil {
		return err
	}
	slices.SortFunc(dirents, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, d := range dirents {
		if err := ctx.Err(); err != nil {
			return err
		}
		p := filepath.Join(real, d.Name())
		info, err := d.Info()
		if err != nil {
			continue
		}
		e := Entry{
			Path: path.Join(api, d.Name()), Real: p, Name: d.Name(), Dir: info.IsDir(),
			MTime: info.ModTime(), Hidden: ccfs.HiddenEntry(d), Mode: modeBits(info.Mode()),
		}
		link := info.Mode()&fs.ModeSymlink != 0
		if link {
			if t, err := os.Stat(p); err == nil {
				e.Dir, info = t.IsDir(), t
			}
		}
		if !e.Dir {
			e.Size = info.Size()
		}
		err = fn(e)
		if errors.Is(err, fs.SkipDir) {
			continue
		}
		if err != nil {
			return err
		}
		// A link to a folder is listed but not entered, so a walk cannot
		// run in circles or out of its source.
		if !e.Dir || link {
			continue
		}
		if err := walkDisk(ctx, p, e.Path, fn); err != nil && !isReadError(err) {
			return err
		}
	}
	return nil
}

func isReadError(err error) bool {
	var pe *fs.PathError
	return errors.As(err, &pe)
}

func modeBits(m fs.FileMode) uint32 {
	bits := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		bits |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		bits |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		bits |= 0o1000
	}
	return bits
}
