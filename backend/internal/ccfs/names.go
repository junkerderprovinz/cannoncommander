package ccfs

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// BinName is the trash folder of a share, PartSuffix marks a file that is still
// being written.
const (
	BinName    = ".cc-trash"
	PartSuffix = ".cc-part"
)

// Reserved reports the names the app keeps for itself. Nobody gets at them
// through a path, because a bin holds other people's files and a part file is
// written to without another look at its rights.
func Reserved(name string) bool {
	return name == BinName || strings.HasPrefix(name, ".") && strings.HasSuffix(name, PartSuffix)
}

// dataMarker marks the folder of the app's data, so the folder can be told apart
// when a source shows it under another path than the app knows it by.
const dataMarker = ".cc-data"

// WithDataFolder returns a copy of r that keeps everybody out of dir, the folder
// that holds the database and the config. Whoever could read it would have the
// API tokens and two factor secrets of all users, and could write in a config
// that turns the sign in off. A container sees this folder as /config, while a
// share on the same disk usually shows it under appdata, so the places a source
// usually has it are searched for the marker, and the mounts of the container
// tell where else it shows up.
func (r *Resolver) WithDataFolder(dir string) *Resolver {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if err := os.WriteFile(filepath.Join(dir, dataMarker), nil, 0o600); err != nil {
		logger.Warningf("cannot mark the data folder: %v", err)
	}
	places := []string{dir}
	if data, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		places = append(places, mountedElsewhere(string(data), dir)...)
	}
	out := *r
	out.closed = nil
	for _, s := range r.sources {
		for _, p := range places {
			if rel, err := filepath.Rel(s.Root, p); err == nil && rel != "." && inside(s.Root, p) {
				out.closed = append(out.closed, path.Join(s.Mount, filepath.ToSlash(rel)))
			}
		}
		// Looking into a disk for the marker would wake it. A disk or pool
		// holds the folder where the user shares have it.
		if s.Device {
			continue
		}
		for _, rel := range markedFolders(s.Root) {
			out.closed = append(out.closed, path.Join(s.Mount, rel))
		}
	}
	out.closed = append(out.closed, r.onEveryVolume(out.closed)...)
	slices.Sort(out.closed)
	out.closed = slices.Compact(out.closed)
	return &out
}

// onEveryVolume returns the paths in the user shares and on each disk and pool
// for those of paths that lie in one of them, since a share folder has the
// same path on all of them.
func (r *Resolver) onEveryVolume(paths []string) []string {
	var volumes []string
	for _, s := range r.sources {
		if s.Device || s.Mount == "/mnt/user" {
			volumes = append(volumes, s.Mount)
		}
	}
	var out []string
	for _, p := range paths {
		for _, v := range volumes {
			rest, ok := strings.CutPrefix(p, v+"/")
			if !ok {
				continue
			}
			for _, w := range volumes {
				out = append(out, path.Join(w, rest))
			}
		}
	}
	return out
}

// mountedElsewhere reads the mount table in mountinfo and returns the other
// paths where the folder dir can be reached, such as /mnt/cache/appdata/x for a
// /config that the container got from that folder.
func mountedElsewhere(mountinfo, dir string) []string {
	type mount struct{ dev, root, at string }
	var mounts []mount
	for line := range strings.SplitSeq(mountinfo, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		mounts = append(mounts, mount{dev: f[2], root: unescapeMount(f[3]), at: unescapeMount(f[4])})
	}
	var home *mount
	for i, m := range mounts {
		if within(dir, m.at) && (home == nil || len(m.at) >= len(home.at)) {
			home = &mounts[i]
		}
	}
	if home == nil {
		return nil
	}
	inFs := path.Join(home.root, strings.TrimPrefix(dir, home.at))
	var out []string
	for _, m := range mounts {
		if m.dev != home.dev || m.at == home.at || !within(inFs, m.root) {
			continue
		}
		p := path.Join(m.at, strings.TrimPrefix(inFs, m.root))
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// unescapeMount undoes the octal escapes mountinfo writes for spaces, tabs,
// newlines and backslashes in a path.
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

// markedFolders looks in the folders directly in root and in those of its
// appdata, where a container's data lives on Unraid.
func markedFolders(root string) []string {
	var found []string
	for _, parent := range []string{"", "appdata"} {
		entries, err := os.ReadDir(filepath.Join(root, parent))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if _, err := os.Stat(filepath.Join(root, parent, e.Name(), dataMarker)); err == nil {
				found = append(found, path.Join(parent, e.Name()))
			}
		}
	}
	return found
}

// isClosed reports whether p lies in the app's data.
func (r *Resolver) isClosed(p string) bool {
	if len(r.closed) == 0 {
		return false
	}
	outer, _, _ := strings.Cut(p, ArchiveSep)
	return slices.ContainsFunc(r.closed, func(c string) bool { return within(outer, c) })
}

// holdsClosed reports whether the app's data lies below p.
func (r *Resolver) holdsClosed(p string) bool {
	return slices.ContainsFunc(r.closed, func(c string) bool { return within(c, p) })
}

func reservedPath(p string) bool {
	if !strings.Contains(p, ".cc-") {
		return false
	}
	outer, _, _ := strings.Cut(p, ArchiveSep)
	for seg := range strings.SplitSeq(outer, "/") {
		if Reserved(seg) {
			return true
		}
	}
	return false
}

// FreeName returns name, or when dir already holds an entry by that name
// the first free "name (n).ext", as the prototype names a second copy.
func FreeName(dir, name string) string {
	if !exists(filepath.Join(dir, name)) {
		return name
	}
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	for n := 1; ; n++ {
		c := base + " (" + strconv.Itoa(n) + ")" + ext
		if !exists(filepath.Join(dir, c)) {
			return c
		}
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// ValidName reports whether name can be one entry in a folder.
func ValidName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		!strings.ContainsAny(name, "/\x00") && !strings.Contains(name, ArchiveSep)
}

// OpenRegular opens a file for reading if it is a plain file, for walks over
// what the index knows, where a link could be anything by now.
func OpenRegular(name string) (*os.File, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = ErrEscape
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// OpenPart opens a part file for writing. A part file gets written without a
// new check of the path, so a link in its place would pass the bytes on to
// whatever it points at.
func OpenPart(name string, flag int) (*os.File, error) {
	f, err := os.OpenFile(name, flag|noFollow, 0o666)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || !singleLink(info)) {
		err = ErrEscape
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
