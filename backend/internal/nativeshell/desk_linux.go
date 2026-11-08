package nativeshell

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// desk is what the desktop knows, read again after a while so that a
// change in its settings shows up.
type desk struct {
	xdg.Desktop
	db           *xdg.MimeDB
	theme        string
	icons        *xdg.Icons
	userDirs     map[string]string
	thumbnailers []xdg.Thumbnailer
	read         time.Time
	// root is the file system /proc and /sys are read from, and hasOwner
	// asks the session bus about a name. Tests replace both.
	root     fs.FS
	hasOwner func(name string) bool
}

var (
	deskMu sync.Mutex
	shared *desk
)

// current is the desktop of this process, read at most once a minute.
func current() *desk {
	deskMu.Lock()
	defer deskMu.Unlock()
	if shared == nil || time.Since(shared.read) > time.Minute {
		shared = newDesk(xdg.Current(), shared)
	}
	return shared
}

// newDesk reads d. It keeps the icons of last while the theme stays the
// same, since they remember every folder they have looked into.
func newDesk(d xdg.Desktop, last *desk) *desk {
	k := &desk{Desktop: d, db: d.MimeDB(), theme: d.ThemeName(), userDirs: d.UserDirs(), thumbnailers: d.Thumbnailers(), read: time.Now(),
		root: os.DirFS("/"), hasOwner: nameHasOwner,
	}
	if last != nil && last.theme == k.theme {
		k.icons = last.icons
	} else {
		k.icons = d.Icons(k.theme)
	}
	return k
}

// item is one of the files a menu is for.
type item struct {
	path string
	dir  bool
	mime string
}

// selection is the files a menu is for, all in one folder.
type selection struct {
	dir   string
	items []item
}

func (k *desk) selection(files []string) (selection, error) {
	if len(files) == 0 {
		return selection{}, errors.New("no files")
	}
	sel := selection{dir: filepath.Dir(files[0])}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return selection{}, err
		}
		it := item{path: f, dir: info.IsDir(), mime: "inode/directory"}
		if !it.dir {
			it.mime = k.db.TypeOf(f)
		}
		sel.items = append(sel.items, it)
	}
	return sel, nil
}

func (s selection) paths() []string {
	out := make([]string, len(s.items))
	for i, it := range s.items {
		out[i] = it.path
	}
	return out
}

func (s selection) uris() []string {
	out := make([]string, len(s.items))
	for i, it := range s.items {
		out[i] = xdg.FileURI(it.path)
	}
	return out
}

// is reports whether it is of type pattern: that type, one it is a kind
// of, or a wildcard such as image/*.
func (k *desk) is(it item, pattern string) bool {
	if pattern == "*" || pattern == "*/*" {
		return true
	}
	pattern = k.db.Canonical(pattern)
	media, wild := strings.CutSuffix(pattern, "/*")
	for _, m := range k.db.Lineage(it.mime) {
		if m == pattern || wild && strings.HasPrefix(m, media+"/") {
			return true
		}
	}
	return false
}

func executable(file string) bool {
	info, err := os.Stat(file)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
