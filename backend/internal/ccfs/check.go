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
	return r.check(u, p, perm, false)
}

// CheckCarried is Check for an entry that goes along with its folder, where a
// pattern that hides by name alone does not count.
func (r *Resolver) CheckCarried(u *users.User, p string, perm Perm) (Location, error) {
	return r.check(u, p, perm, true)
}

func (r *Resolver) check(u *users.User, p string, perm Perm, carried bool) (Location, error) {
	loc, err := r.Resolve(p)
	if err != nil {
		return Location{}, err
	}
	if u == nil {
		return Location{}, ErrForbidden
	}
	if reservedPath(loc.Path) || r.isClosed(loc.Path) {
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
	if !holds(granted(u, loc.Source), perm) {
		return Location{}, ErrForbidden
	}
	var effect Effect
	if carried {
		effect = r.carriedEffect(u, loc.Path)
	} else {
		effect = r.effect(u, loc.Path)
	}
	switch effect {
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
	if u == nil || Reserved(name) || r.isClosed(path.Join(dir.Path, name)) {
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

// Landed is the location of a folder once its links are followed. Whatever is
// judged below a folder has to go by this path, since a rule cannot name the
// path of a link that leads there.
func (r *Resolver) Landed(u *users.User, p string, perm Perm) (Location, error) {
	loc, err := r.Reach(u, p, perm)
	if err != nil || loc.Virtual() || loc.InArchive() {
		return loc, err
	}
	return r.Followed(loc)
}

// Entry checks an entry that gets taken as itself, so a link counts as a link
// and can go away wherever it leads. Only the folders on the way are followed,
// and the location it returns is the one that leaves.
func (r *Resolver) Entry(u *users.User, p string, perm Perm) (Location, error) {
	return r.entry(u, p, perm, false)
}

// EntryCarried is Entry for something the app itself took along with its folder
// and now has to find again.
func (r *Resolver) EntryCarried(u *users.User, p string, perm Perm) (Location, error) {
	return r.entry(u, p, perm, true)
}

func (r *Resolver) entry(u *users.User, p string, perm Perm, carried bool) (Location, error) {
	loc, err := r.check(u, p, perm, carried)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
		return loc, r.Follows(u, loc, perm)
	}
	landed, err := r.entryAt(loc)
	if err != nil || landed.Path == loc.Path {
		return loc, err
	}
	return r.check(u, landed.Path, perm, carried)
}

// entryAt is loc with the links on the way to it followed, but not the last.
func (r *Resolver) entryAt(loc Location) (Location, error) {
	dir, err := r.Resolve(path.Dir(loc.Path))
	if err != nil {
		return loc, err
	}
	landed, err := r.Followed(dir)
	if err != nil {
		return loc, err
	}
	return r.Resolve(path.Join(landed.Path, path.Base(loc.Path)))
}

// belowRules is what a rule book can say about a whole subtree without
// looking at the disk.
type belowRules interface {
	Below(u *users.User, p string) bool
}

// carryRules is what a rule book can say about entries that go along with a
// folder. Whatever hides by name alone goes with it, as in other file managers,
// while a rule on a path protects somebody's data from being taken through a
// parent.
type carryRules interface {
	CarryEffect(u *users.User, p string) Effect
	CarryBelow(u *users.User, p string) bool
}

// Whole checks perm on everything below loc, for an operation that hands the
// folder to something that cannot leave an entry out. A rule can name one entry
// deep inside a folder. Links are not followed, since such operations take them
// as links.
func (r *Resolver) Whole(ctx context.Context, u *users.User, loc Location, perm Perm) error {
	if loc.Virtual() || loc.InArchive() {
		return nil
	}
	top, err := r.entryAt(loc)
	if err != nil {
		return err
	}
	if r.holdsClosed(top.Path) {
		return ErrProtected
	}
	if r.rules == nil {
		return nil
	}
	if b, ok := r.rules.(belowRules); ok && !b.Below(u, top.Path) {
		return nil
	}
	return r.walk(ctx, u, top, perm, false)
}

// Carry checks that loc can be deleted, moved or renamed with all it holds. An
// entry hidden by a pattern on its name goes along, while a rule on a path keeps
// the folder where it is, and so does a read rule of either kind.
func (r *Resolver) Carry(ctx context.Context, u *users.User, loc Location, perm Perm) error {
	b, ok := r.rules.(carryRules)
	if !ok {
		return r.Whole(ctx, u, loc, perm)
	}
	if loc.Virtual() || loc.InArchive() {
		return nil
	}
	top, err := r.entryAt(loc)
	if err != nil {
		return err
	}
	if r.holdsClosed(top.Path) {
		return ErrProtected
	}
	if !b.CarryBelow(u, top.Path) {
		return nil
	}
	return r.walk(ctx, u, top, perm, true)
}

func (r *Resolver) walk(ctx context.Context, u *users.User, top Location, perm Perm, carried bool) error {
	return filepath.WalkDir(top.Real, func(real string, d fs.DirEntry, err error) error {
		// An entry that went away meanwhile needs no right.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
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
		rel, err := filepath.Rel(top.Real, real)
		if err != nil {
			return err
		}
		if _, err := r.check(u, path.Join(top.Path, filepath.ToSlash(rel)), perm, carried); err != nil {
			return ErrProtected
		}
		return nil
	})
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

// carriedEffect is effect for an entry that goes along with its folder.
func (r *Resolver) carriedEffect(u *users.User, p string) Effect {
	b, ok := r.rules.(carryRules)
	if !ok {
		return r.effect(u, p)
	}
	if outer, _, found := strings.Cut(p, ArchiveSep); found {
		p = outer
	}
	return b.CarryEffect(u, p)
}

// reaches reports whether some place u may open lies at or below p, or p
// lies inside such a place.
func (r *Resolver) reaches(u *users.User, p string) bool {
	for _, s := range r.current().sources {
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

// scope returns the folder inside s that u is confined to. An admin who was
// never given a scope in s has all of it, so a source added later and the
// disks and pools reach the admins without another round in the user list.
func (r *Resolver) scope(u *users.User, s *Source) (string, bool) {
	if s.AdminOnly && !u.Permissions.Admin {
		return "", false
	}
	for _, bs := range u.BackendScopes {
		if bs.Path == s.Root {
			return path.Clean("/" + bs.Scope), true
		}
	}
	return "/", u.Permissions.Admin
}

// granted returns the rights u holds in s, all of them for an admin without
// a scope there.
func granted(u *users.User, s *Source) users.SourceFilePermissions {
	g, ok := u.FilePermsForSourcePath(s.Root)
	if !ok && u.Permissions.Admin {
		return users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	}
	return g
}

func holds(g users.SourceFilePermissions, perm Perm) bool {
	return (perm&View == 0 || g.View) &&
		(perm&Download == 0 || g.Download) &&
		(perm&Create == 0 || g.Create) &&
		(perm&Modify == 0 || g.Modify) &&
		(perm&Delete == 0 || g.Delete)
}
