// Package ccfs maps the absolute paths of the API onto the configured
// sources and decides who may do what with them. Inside the container the
// paths equal the host paths, so /mnt/user/filme means the same thing in
// the browser, in the container and on the Unraid console.
package ccfs

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// ArchiveSep separates an archive file from the path inside it, as in
// /mnt/user/downloads/fotos.zip::/Strand/IMG_0101.jpg.
const ArchiveSep = "::"

var (
	ErrBadPath   = errors.New("bad path")
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrEscape    = errors.New("path leaves its source")
	ErrReadOnly  = errors.New("read only")
	// ErrProtected refuses an operation on a folder because of what is in it.
	ErrProtected = errors.New("a rule protects something in this folder")
)

// Source is one configured root. Mount is where it shows up in the API,
// Root is the directory on disk. In the container both are the same path.
type Source struct {
	Name      string
	Mount     string
	Root      string
	ReadOnly  bool
	AdminOnly bool
}

// Location is a resolved API path.
type Location struct {
	// Path is the cleaned API path, archive part included.
	Path string
	// Source is nil for a virtual folder such as / or /mnt.
	Source *Source
	// Index is the path inside the source, starting with "/".
	Index string
	// Real is the file on disk. For a path inside an archive it is the
	// archive file itself.
	Real string
	// Inner is the path inside the archive, "/" for its root, or "" when
	// the location is not inside an archive.
	Inner string
}

// Virtual reports whether the location is a folder that only exists to
// lead to the sources below it.
func (l Location) Virtual() bool { return l.Source == nil }

// InArchive reports whether the location points into an archive.
func (l Location) InArchive() bool { return l.Inner != "" }

// Resolver turns API paths into locations. It is immutable once built.
type Resolver struct {
	sources []*Source
	rules   Rules
	// devices are the pools and disks Unraid reports, for pools that are
	// not called cache.
	devices []string
}

// New builds a resolver. Mounts must be absolute and may not nest inside
// each other, because then one path would belong to two sources.
func New(rules Rules, sources ...Source) (*Resolver, error) {
	r := &Resolver{rules: rules}
	for _, s := range sources {
		mount, err := Clean(s.Mount)
		if err != nil || mount == "/" {
			return nil, errors.New("source " + s.Name + " needs an absolute mount below /")
		}
		s.Mount = mount
		s.Root = filepath.Clean(s.Root)
		s.AdminOnly = s.AdminOnly || r.AdminOnly(mount)
		for _, o := range r.sources {
			if within(mount, o.Mount) || within(o.Mount, mount) {
				return nil, errors.New("sources " + o.Name + " and " + s.Name + " overlap")
			}
		}
		r.sources = append(r.sources, &s)
	}
	slices.SortFunc(r.sources, func(a, b *Source) int { return strings.Compare(a.Mount, b.Mount) })
	return r, nil
}

// WithDevices returns a copy of r that also keeps the named pools and disks
// for admins, since a pool may go by any name.
func (r *Resolver) WithDevices(names ...string) *Resolver {
	out := &Resolver{rules: r.rules, devices: slices.Clone(names)}
	for _, s := range r.sources {
		c := *s
		c.AdminOnly = c.AdminOnly || out.AdminOnly(c.Mount)
		out.sources = append(out.sources, &c)
	}
	return out
}

// Sources returns the configured sources sorted by mount.
func (r *Resolver) Sources() []Source {
	out := make([]Source, len(r.sources))
	for i, s := range r.sources {
		out[i] = *s
	}
	return out
}

// Clean checks an API path and returns its canonical form. Paths have to be
// absolute and may not contain "..", a NUL byte or an empty archive part.
// The root of an archive comes back as /a.zip::/.
func Clean(p string) (string, error) {
	if p == "" || p[0] != '/' || strings.ContainsRune(p, 0) {
		return "", ErrBadPath
	}
	outer, inner, hasInner := strings.Cut(p, ArchiveSep)
	if hasInner && strings.Contains(inner, ArchiveSep) {
		return "", ErrBadPath
	}
	for _, part := range []string{outer, inner} {
		for _, seg := range strings.Split(part, "/") {
			if seg == ".." {
				return "", ErrBadPath
			}
		}
	}
	outer = path.Clean(outer)
	if !hasInner {
		return outer, nil
	}
	// A colon right before the separator would make it ambiguous where the
	// archive name ends.
	if outer == "/" || strings.HasSuffix(outer, ":") {
		return "", ErrBadPath
	}
	return outer + ArchiveSep + path.Clean("/"+inner), nil
}

// Resolve maps an API path to its location without checking any rights.
func (r *Resolver) Resolve(p string) (Location, error) {
	clean, err := Clean(p)
	if err != nil {
		return Location{}, err
	}
	loc := Location{Path: clean}
	outer, inner, hasInner := strings.Cut(clean, ArchiveSep)
	if hasInner {
		loc.Inner = inner
	}
	for _, s := range r.sources {
		if !within(outer, s.Mount) {
			continue
		}
		loc.Source = s
		loc.Index = "/" + strings.TrimPrefix(strings.TrimPrefix(outer, s.Mount), "/")
		loc.Real = filepath.Join(s.Root, filepath.FromSlash(loc.Index))
		// On Windows a backslash in a name is a separator, so "\.." could
		// still climb out after the checks in Clean.
		if !inside(s.Root, loc.Real) {
			return Location{}, ErrBadPath
		}
		return loc, nil
	}
	if hasInner || !r.isVirtual(outer) {
		return Location{}, ErrNotFound
	}
	return loc, nil
}

// VirtualChildren lists the names directly below a virtual folder, which
// are further virtual folders or the mounts of sources.
func (r *Resolver) VirtualChildren(dir string) []string {
	var names []string
	for _, s := range r.sources {
		if !within(s.Mount, dir) || s.Mount == dir {
			continue
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(s.Mount, dir), "/")
		name, _, _ := strings.Cut(rest, "/")
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

func (r *Resolver) isVirtual(p string) bool {
	for _, s := range r.sources {
		if within(s.Mount, p) && s.Mount != p {
			return true
		}
	}
	return false
}

// Contained follows the symlinks in loc.Real and reports ErrEscape when the
// target lies outside the location's source. Parts that do not exist yet
// are taken as they are, so a path that is about to be created passes.
func Contained(loc Location) error {
	if loc.Source == nil {
		return nil
	}
	root, err := filepath.EvalSymlinks(loc.Source.Root)
	if err != nil {
		return err
	}
	real, err := evalExisting(loc.Real)
	if err != nil {
		return err
	}
	if !inside(root, real) {
		return ErrEscape
	}
	return nil
}

// inside reports whether the file path p is root or lies below it.
func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// evalExisting resolves symlinks in the longest existing prefix of p and
// appends the rest unchanged.
func evalExisting(p string) (string, error) {
	var tail []string
	cur := p
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		// An entry that exists but cannot be resolved is a dangling link.
		// Writing through it would create its target, wherever that is.
		if _, lerr := os.Lstat(cur); lerr == nil {
			return "", ErrEscape
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}

// within reports whether p is base or lies below it.
func within(p, base string) bool {
	if base == "/" {
		return true
	}
	return p == base || strings.HasPrefix(p, base+"/")
}

// Followed returns the location loc ends up at once its symlinks are
// followed. Rules name paths, so a link has to answer for the place it leads
// to as well. Like Contained it reports ErrEscape for a target outside the
// source and takes parts that do not exist yet as they are.
func (r *Resolver) Followed(loc Location) (Location, error) {
	if loc.Source == nil {
		return loc, nil
	}
	root, err := filepath.EvalSymlinks(loc.Source.Root)
	if err != nil {
		return Location{}, err
	}
	real, err := evalExisting(loc.Real)
	if err != nil {
		return Location{}, err
	}
	if !inside(root, real) {
		return Location{}, ErrEscape
	}
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return Location{}, err
	}
	return r.Resolve(path.Join(loc.Source.Mount, filepath.ToSlash(rel)))
}
