package ccfs

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
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
	if reservedPath(loc.Path) {
		if perm&writePerms != 0 {
			return Location{}, ErrForbidden
		}
		return Location{}, ErrNotFound
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
	if u == nil || Reserved(name) {
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

// Follows checks the place loc really is once its symlinks are followed, so a
// link cannot lead past the source, the user's scope or a rule. Check only
// reads the path, and many callers ask it for every entry of a walk, which
// would be too slow with a look at the disk for each.
func (r *Resolver) Follows(u *users.User, loc Location, perm Perm) error {
	target, err := r.Followed(loc)
	if err != nil {
		return err
	}
	outer, _, _ := strings.Cut(loc.Path, ArchiveSep)
	if target.Path == outer {
		return nil
	}
	_, err = r.Check(u, target.Path, perm)
	return err
}

// Reach is Check and Follows in one, for a path that is about to be used.
func (r *Resolver) Reach(u *users.User, p string, perm Perm) (Location, error) {
	loc, err := r.Check(u, p, perm)
	if err != nil {
		return loc, err
	}
	return loc, r.Follows(u, loc, perm)
}

// belowRules is what a rule book can say about a whole subtree without
// looking at the disk.
type belowRules interface {
	Below(u *users.User, p string) bool
}

// Whole checks perm on everything below loc. A rule can name one entry deep
// inside a folder, and an operation on the folder would take that entry past
// its rule. Links are not followed, since such operations take them as links.
func (r *Resolver) Whole(ctx context.Context, u *users.User, loc Location, perm Perm) error {
	if r.rules == nil || loc.Virtual() || loc.InArchive() {
		return nil
	}
	if b, ok := r.rules.(belowRules); ok && !b.Below(u, loc.Path) {
		return nil
	}
	err := filepath.WalkDir(loc.Real, func(real string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// The resolver cannot name such an entry, so no rule can either.
		if Reserved(d.Name()) || strings.Contains(d.Name(), ArchiveSep) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(loc.Real, real)
		if err != nil {
			return err
		}
		_, err = r.Check(u, path.Join(loc.Path, filepath.ToSlash(rel)), perm)
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
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
