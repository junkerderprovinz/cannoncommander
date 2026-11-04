package ccfs

import (
	"path"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Perm is one right on a path. Several can be combined with |.
type Perm uint8

const (
	View Perm = 1 << iota
	Download
	Create
	Modify
	Delete
)

const writePerms = Create | Modify | Delete

// Effect is what the rules of the app say about one path for one user.
type Effect uint8

const (
	Open Effect = iota
	ReadOnly
	Hidden
)

// Rules are the ordered access rules an admin keeps in the app. They work
// on API paths and can hide a place or make it read only.
type Rules interface {
	Effect(u *users.User, p string) Effect
}

// Check resolves p and makes sure u holds every right in perm there.
//
// A user sees the virtual folders and the folders above their scope only
// as far as they lead to something they may open, and only for reading.
func (r *Resolver) Check(u *users.User, p string, perm Perm) (Location, error) {
	loc, err := r.Resolve(p)
	if err != nil {
		return Location{}, err
	}
	if u == nil {
		return Location{}, ErrForbidden
	}
	if loc.Virtual() {
		if perm != View || !r.reaches(u, loc.Path) {
			return Location{}, ErrForbidden
		}
		return loc, nil
	}
	scope, ok := r.scope(u, loc.Source)
	if !ok || !u.Permissions.Admin && r.AdminOnly(loc.Path) {
		return Location{}, ErrForbidden
	}
	if within(scope, loc.Index) && scope != loc.Index {
		if perm != View {
			return Location{}, ErrForbidden
		}
		return loc, nil
	}
	if !within(loc.Index, scope) {
		return Location{}, ErrForbidden
	}
	if perm&writePerms != 0 && (loc.Source.ReadOnly || loc.InArchive()) {
		return Location{}, ErrReadOnly
	}
	granted, _ := u.FilePermsForSourcePath(loc.Source.Root)
	if !holds(granted, perm) {
		return Location{}, ErrForbidden
	}
	switch r.effect(u, loc.Path) {
	case Hidden:
		return Location{}, ErrForbidden
	case ReadOnly:
		if perm&writePerms != 0 {
			return Location{}, ErrReadOnly
		}
	}
	return loc, nil
}

// Visible reports whether u may see the entry name inside the folder dir,
// which the caller already passed through Check.
func (r *Resolver) Visible(u *users.User, dir Location, name string) bool {
	if u == nil {
		return false
	}
	if dir.Virtual() {
		return r.reaches(u, path.Join(dir.Path, name))
	}
	if dir.InArchive() {
		return true
	}
	scope, ok := r.scope(u, dir.Source)
	if !ok || !u.Permissions.Admin && r.AdminOnly(path.Join(dir.Path, name)) {
		return false
	}
	child := path.Join(dir.Index, name)
	if !within(child, scope) {
		return within(scope, child)
	}
	return r.effect(u, path.Join(dir.Path, name)) != Hidden
}

// effect asks the rules about p. A path into an archive counts as the
// archive file, which is what the rules can name.
func (r *Resolver) effect(u *users.User, p string) Effect {
	if r.rules == nil {
		return Open
	}
	if outer, _, found := strings.Cut(p, ArchiveSep); found {
		p = outer
	}
	return r.rules.Effect(u, p)
}

// reaches reports whether some place u may open lies at or below p, or p
// lies inside such a place.
func (r *Resolver) reaches(u *users.User, p string) bool {
	for _, s := range r.sources {
		scope, ok := r.scope(u, s)
		if !ok {
			continue
		}
		top := path.Join(s.Mount, scope)
		if within(top, p) || within(p, top) {
			return true
		}
	}
	return false
}

// scope returns the folder inside s that u is confined to.
func (r *Resolver) scope(u *users.User, s *Source) (string, bool) {
	if s.AdminOnly && !u.Permissions.Admin {
		return "", false
	}
	for _, bs := range u.BackendScopes {
		if bs.Path == s.Root {
			return path.Clean("/" + bs.Scope), true
		}
	}
	return "", false
}

func holds(g users.SourceFilePermissions, perm Perm) bool {
	return (perm&View == 0 || g.View) &&
		(perm&Download == 0 || g.Download) &&
		(perm&Create == 0 || g.Create) &&
		(perm&Modify == 0 || g.Modify) &&
		(perm&Delete == 0 || g.Delete)
}
