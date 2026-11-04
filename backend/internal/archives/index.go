package archives

import (
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Index is the table of contents of an archive, so it can be browsed like
// a folder with the archive::/inner paths of the API.
type Index struct {
	entries map[string]Entry
	kids    map[string][]string
}

// Children lists what lies directly inside the folder inner, "/" for the
// top. Folders that the archive only implies by the names below them are
// there too.
func (ix *Index) Children(inner string) ([]Entry, bool) {
	dir := strings.Trim(inner, "/")
	if dir != "" {
		if e, ok := ix.entries[dir]; !ok || !e.Dir {
			return nil, false
		}
	}
	names := ix.kids[dir]
	out := make([]Entry, 0, len(names))
	for _, n := range names {
		out = append(out, ix.entries[n])
	}
	return out, true
}

// Stat returns the entry at inner.
func (ix *Index) Stat(inner string) (Entry, bool) {
	name := strings.Trim(inner, "/")
	if name == "" {
		return Entry{Dir: true}, true
	}
	e, ok := ix.entries[name]
	return e, ok
}

// Len is the number of entries, implied folders included.
func (ix *Index) Len() int { return len(ix.entries) }

func newIndex(a Archive) (*Index, error) {
	ix := &Index{entries: map[string]Entry{}, kids: map[string][]string{}}
	add := func(e Entry) {
		if old, ok := ix.entries[e.Name]; ok {
			// A later entry of the same name wins, as it does when unpacking,
			// but a folder with entries below it stays a folder.
			e.Dir = e.Dir || old.Dir
			ix.entries[e.Name] = e
			return
		}
		ix.entries[e.Name] = e
		parent := path.Dir(e.Name)
		if parent == "." {
			parent = ""
		}
		ix.kids[parent] = append(ix.kids[parent], e.Name)
	}
	err := a.Walk(func(e Entry, _ Opener) error {
		for dir := path.Dir(e.Name); dir != "."; dir = path.Dir(dir) {
			if _, ok := ix.entries[dir]; ok {
				break
			}
			add(Entry{Name: dir, Dir: true, Mode: 0o755 | os.ModeDir})
		}
		add(e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for k := range ix.kids {
		sort.Strings(ix.kids[k])
	}
	return ix, nil
}

type cacheKey struct {
	file, password string
	size           int64
	mtime          time.Time
}

const cacheSize = 16

var (
	cacheMu sync.Mutex
	cache   = map[cacheKey]*Index{}
	order   []cacheKey
)

// Load returns the index of the archive at file. Indexes of recently
// browsed archives are kept while the file stays unchanged.
func Load(file string, o Options) (*Index, error) {
	st, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	key := cacheKey{file, o.Password, st.Size(), st.ModTime()}
	cacheMu.Lock()
	ix, ok := cache[key]
	cacheMu.Unlock()
	if ok {
		return ix, nil
	}
	a, err := Open(file, o)
	if err != nil {
		return nil, err
	}
	defer a.Close()
	if ix, err = newIndex(a); err != nil {
		return nil, err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if _, ok := cache[key]; !ok {
		cache[key] = ix
		order = append(order, key)
		if len(order) > cacheSize {
			delete(cache, order[0])
			order = order[1:]
		}
	}
	return ix, nil
}

var errFound = errors.New("found")

// ReadEntry hands the content of the file entry inner to fn. It returns
// fs.ErrNotExist when there is no such file.
func ReadEntry(file string, o Options, inner string, fn func(e Entry, r io.Reader) error) error {
	name := strings.Trim(inner, "/")
	a, err := Open(file, o)
	if err != nil {
		return err
	}
	defer a.Close()
	var ferr error
	err = a.Walk(func(e Entry, open Opener) error {
		if e.Name != name || e.Dir || e.Symlink() || e.HardLink != "" {
			return nil
		}
		rc, err := open()
		if err != nil {
			return err
		}
		defer rc.Close()
		ferr = fn(e, rc)
		return errFound
	})
	if errors.Is(err, errFound) {
		return ferr
	}
	if err != nil {
		return err
	}
	return os.ErrNotExist
}
