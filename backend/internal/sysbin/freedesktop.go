//go:build linux

package sysbin

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// freedesktop is the trash of the freedesktop.org specification, which
// GNOME, KDE, Xfce and Cinnamon share: one in the home folder and one at the
// top of every other volume.
type freedesktop struct {
	// home is the trash for everything on the file system of the home
	// folder's data directory.
	home   string
	uid    int
	mounts func() ([]string, error)
}

// Open returns the trash of the person running the app.
func Open() (Bin, error) {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		data = filepath.Join(home, ".local", "share")
	}
	return &freedesktop{home: filepath.Join(data, "Trash"), uid: os.Getuid(), mounts: procMounts}, nil
}

// place is one trash folder. top is the volume it serves, empty for the
// trash in the home folder, whose records hold absolute paths.
type place struct {
	dir, top string
}

func (p place) key() string {
	if p.top == "" {
		return "h"
	}
	h := fnv.New32a()
	h.Write([]byte(p.dir))
	return fmt.Sprintf("%08x", h.Sum32())
}

func (p place) info(name string) string {
	return filepath.Join(p.dir, "info", name+".trashinfo")
}

func (b *freedesktop) Put(path string) (Entry, error) {
	return b.put(path, path, time.Now())
}

func (b *freedesktop) Adopt(path, origin string, deleted time.Time) (Entry, error) {
	return b.put(path, origin, deleted)
}

func (b *freedesktop) put(path, origin string, deleted time.Time) (Entry, error) {
	if _, err := os.Lstat(path); err != nil {
		return Entry{}, err
	}
	pl, err := b.placeFor(path)
	if err != nil {
		return Entry{}, err
	}
	recorded := origin
	if pl.top != "" {
		rel, err := filepath.Rel(pl.top, origin)
		if err != nil || !filepath.IsLocal(rel) {
			return Entry{}, ErrNoBin
		}
		recorded = rel
	}
	deleted = deleted.Truncate(time.Second)
	name, err := reserve(pl, filepath.Base(origin), formatTrashInfo(recorded, deleted))
	if err != nil {
		return Entry{}, err
	}
	if err := os.Rename(path, filepath.Join(pl.dir, "files", name)); err != nil {
		os.Remove(pl.info(name))
		if errors.Is(err, syscall.EXDEV) {
			return Entry{}, ErrNoBin
		}
		return Entry{}, err
	}
	return b.entry(pl, name, nil)
}

// reserve claims a name in the trash by creating its info file, which the
// specification makes the lock: whoever creates it first owns the name.
func reserve(pl place, base string, info []byte) (string, error) {
	stem, ext := base, ""
	if dot := strings.LastIndex(base, "."); dot > 0 {
		stem, ext = base[:dot], base[dot:]
	}
	for n := 1; n < 10000; n++ {
		name := base
		if n > 1 {
			name = stem + "." + strconv.Itoa(n) + ext
		}
		f, err := os.OpenFile(pl.info(name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(filepath.Join(pl.dir, "files", name)); err == nil {
			f.Close()
			os.Remove(pl.info(name))
			continue
		}
		_, err = f.Write(info)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(pl.info(name))
			return "", err
		}
		return name, nil
	}
	return "", fmt.Errorf("no free name for %s in the trash", base)
}

// placeFor picks the trash for path: the one in the home folder when path is
// on the same file system, otherwise one at the top of path's volume, the
// shared .Trash if an administrator set it up and the person's own
// .Trash-uid else.
func (b *freedesktop) placeFor(path string) (place, error) {
	parent := filepath.Dir(path)
	dev, err := devOf(parent)
	if err != nil {
		return place{}, err
	}
	if home, err := devOf(existing(b.home)); err == nil && home == dev {
		pl := place{dir: b.home}
		return pl, ready(pl.dir)
	}
	top := topOf(parent, dev)
	if pl, ok := b.shared(top); ok && ready(pl.dir) == nil && b.owned(pl.dir) {
		return pl, nil
	}
	pl := place{dir: filepath.Join(top, ".Trash-"+strconv.Itoa(b.uid)), top: top}
	if ready(pl.dir) != nil || !b.owned(pl.dir) {
		return place{}, ErrNoBin
	}
	return pl, nil
}

// shared is the folder of this person in $top/.Trash. The specification
// only allows it when .Trash is a real folder with the sticky bit, so nobody
// can take away what another put there.
func (b *freedesktop) shared(top string) (place, bool) {
	st, err := os.Lstat(filepath.Join(top, ".Trash"))
	if err != nil || !st.IsDir() || st.Mode()&fs.ModeSticky == 0 {
		return place{}, false
	}
	return place{dir: filepath.Join(top, ".Trash", strconv.Itoa(b.uid)), top: top}, true
}

// owned reports whether dir is a real folder of this person, so a trash
// somebody else planted is never used.
func (b *freedesktop) owned(dir string) bool {
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && int(sys.Uid) == b.uid
}

func ready(dir string) error {
	for _, sub := range []string{"files", "info"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return err
		}
	}
	return nil
}

// places are the trash folders that exist: the home one first, then those on
// mounted volumes.
func (b *freedesktop) places() []place {
	out := []place{{dir: b.home}}
	mounts, _ := b.mounts()
	seen := map[string]bool{b.home: true}
	for _, top := range mounts {
		candidates := []place{{dir: filepath.Join(top, ".Trash-"+strconv.Itoa(b.uid)), top: top}}
		if pl, ok := b.shared(top); ok {
			candidates = append([]place{pl}, candidates...)
		}
		for _, pl := range candidates {
			if !seen[pl.dir] && b.owned(pl.dir) {
				seen[pl.dir] = true
				out = append(out, pl)
			}
		}
	}
	return out
}

func (b *freedesktop) List() ([]Entry, error) {
	var out []Entry
	for _, pl := range b.places() {
		names, err := os.ReadDir(filepath.Join(pl.dir, "info"))
		if err != nil {
			continue
		}
		sizes := readSizes(pl)
		folders := map[string]bool{}
		for _, n := range names {
			name, ok := strings.CutSuffix(n.Name(), ".trashinfo")
			if !ok {
				continue
			}
			if e, err := b.entry(pl, name, sizes); err == nil {
				out = append(out, e)
				folders[name] = e.Dir
			}
		}
		sizes.save(folders)
	}
	return out, nil
}

func (b *freedesktop) Get(id string) (Entry, error) {
	pl, name, err := b.find(id)
	if err != nil {
		return Entry{}, err
	}
	return b.entry(pl, name, nil)
}

func (b *freedesktop) find(id string) (place, string, error) {
	key, name, err := splitID(id)
	if err != nil {
		return place{}, "", err
	}
	for _, pl := range b.places() {
		if pl.key() == key {
			return pl, name, nil
		}
	}
	return place{}, "", ErrNotFound
}

// entry reads one entry. The size of a folder comes from sizes, or from the
// trash's own cache when sizes is nil.
func (b *freedesktop) entry(pl place, name string, sizes *sizeCache) (Entry, error) {
	raw, err := os.ReadFile(pl.info(name))
	if err != nil {
		return Entry{}, ErrNotFound
	}
	ti, err := parseTrashInfo(raw)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	origin, ok := originOf(pl, ti.Path)
	if !ok {
		return Entry{}, ErrNotFound
	}
	data := filepath.Join(pl.dir, "files", name)
	st, err := os.Lstat(data)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	e := Entry{ID: makeID(pl.key(), name), Name: filepath.Base(origin), Origin: origin, Path: data, Dir: st.IsDir(), Size: st.Size(), Deleted: ti.Deleted}
	if e.Dir {
		if sizes == nil {
			sizes = readSizes(pl)
			defer sizes.save(nil)
		}
		e.Size = sizes.of(name, data, pl.info(name))
	}
	return e, nil
}

// originOf reads the path a record holds. The trash of the home folder
// records absolute paths. That of a volume records them relative to its top,
// and a path that leads elsewhere is refused, since anybody who brings the
// volume could have written it.
func originOf(pl place, recorded string) (string, bool) {
	if pl.top == "" {
		return filepath.Clean(recorded), filepath.IsAbs(recorded)
	}
	if filepath.IsAbs(recorded) || !filepath.IsLocal(recorded) {
		return "", false
	}
	return filepath.Join(pl.top, recorded), true
}

func (b *freedesktop) Restore(id, to string) error {
	pl, name, err := b.find(id)
	if err != nil {
		return err
	}
	e, err := b.entry(pl, name, nil)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return &fs.PathError{Op: "restore", Path: to, Err: fs.ErrExist}
	}
	if err := os.Rename(e.Path, to); err != nil {
		return err
	}
	return removeInfo(pl.info(name))
}

func (b *freedesktop) Forget(id string) error {
	pl, name, err := b.find(id)
	if err != nil {
		return err
	}
	return removeInfo(pl.info(name))
}

func removeInfo(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func devOf(p string) (uint64, error) {
	st, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("no device number")
	}
	return uint64(sys.Dev), nil
}

// existing walks up from p to the first folder that exists, since the home
// trash is only made on the first delete.
func existing(p string) string {
	for {
		if _, err := os.Stat(p); err == nil || filepath.Dir(p) == p {
			return p
		}
		p = filepath.Dir(p)
	}
}

// topOf walks up from dir as long as the parent lies on the same file
// system, which ends at its mount point.
func topOf(dir string, dev uint64) string {
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		if d, err := devOf(parent); err != nil || d != dev {
			return dir
		}
		dir = parent
	}
}

// pseudo are file systems of the kernel that never hold a trash.
var pseudo = map[string]bool{
	"proc": true, "sysfs": true, "cgroup": true, "cgroup2": true, "devpts": true, "mqueue": true,
	"debugfs": true, "tracefs": true, "securityfs": true, "pstore": true, "bpf": true, "configfs": true,
	"fusectl": true, "hugetlbfs": true, "autofs": true, "binfmt_misc": true, "efivarfs": true, "nsfs": true,
}

func procMounts() ([]string, error) {
	raw, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return parseMounts(string(raw)), nil
}

// parseMounts returns the mount points in mountinfo, leaving out the kernel's
// own file systems.
func parseMounts(mountinfo string) []string {
	var out []string
	for line := range strings.SplitSeq(mountinfo, "\n") {
		head, tail, ok := strings.Cut(line, " - ")
		f, t := strings.Fields(head), strings.Fields(tail)
		if !ok || len(f) < 5 || len(t) < 1 || pseudo[t[0]] {
			continue
		}
		out = append(out, unescapeMount(f[4]))
	}
	return out
}

// unescapeMount reads the octal escapes mountinfo writes for spaces and
// other blanks.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
