package search

import (
	"context"
	"io/fs"
	"math"
	"path"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// Params are the criteria of the search window. Q is the name line, which
// may hold field tests.
type Params struct {
	Path   string `json:"path"`
	Q      string `json:"q"`
	Hidden bool   `json:"hidden"`
	Type   string `json:"type,omitempty"`
	Min    string `json:"min,omitempty"`
	Max    string `json:"max,omitempty"`
	When   string `json:"when,omitempty"`
}

// Times of the Geändert filter.
var whenDays = map[string]int{"1": 1, "7": 7, "30": 30, "365": 365}

func validWhen(w string) bool {
	_, ok := whenDays[w]
	return ok || w == "" || w == "any" || w == "old"
}

// metaBatch is how many candidates wait for their metadata together, so
// the cache is asked once for many of them.
const metaBatch = 256

// finder walks the folders of a search and hands every entry that passes
// all tests to hit.
type finder struct {
	fs    *ccfs.Resolver
	user  *users.User
	q     Query
	p     Params
	min   float64
	max   float64
	now   time.Time
	meta  *meta.Service
	tags  func(path, tag string) bool
	sizes func(dir fsindex.Entry) int64
	// readUntil ends the reading of new metadata. After it only what the
	// cache holds counts, so a field search over a whole array still
	// answers.
	readUntil time.Time
	// extra is the content test of a content search.
	extra func(ctx context.Context, r *row) (bool, error)

	scanned int
	pending int
	indexed bool
	batch   []*row
}

func newFinder(fs *ccfs.Resolver, u *users.User, p Params, now time.Time) *finder {
	return &finder{
		fs: fs, user: u, p: p, q: Parse(p.Q, now), now: now,
		min: Bytes(p.Min), max: Bytes(p.Max), meta: meta.Default(),
	}
}

// Bookkeeping reports the names CannonCommander keeps for itself.
func Bookkeeping(name string) bool {
	return name == trash.DirName || (strings.HasPrefix(name, ".") && strings.HasSuffix(name, fileops.PartSuffix))
}

// roots returns the folders to walk for loc. A virtual folder such as /mnt
// stands for the sources mounted below it, without the disks and pools: their
// files are found in the user shares, and walking them would wake the disks.
func (f *finder) roots(loc ccfs.Location) []ccfs.Location {
	if !loc.Virtual() {
		return []ccfs.Location{loc}
	}
	var out []ccfs.Location
	for _, s := range f.fs.Sources() {
		if s.Device || !strings.HasPrefix(s.Mount, strings.TrimSuffix(loc.Path, "/")+"/") {
			continue
		}
		if l, err := f.fs.Check(f.user, s.Mount, ccfs.View); err == nil {
			out = append(out, l)
		}
	}
	return out
}

func (f *finder) run(ctx context.Context, loc ccfs.Location, hit func(*row) error) error {
	for _, root := range f.roots(loc) {
		indexed, err := fsindex.Walk(ctx, root, func(e fsindex.Entry) error { return f.visit(ctx, e, hit) })
		f.indexed = f.indexed || indexed
		if err != nil {
			return err
		}
	}
	return f.flush(ctx, hit)
}

func (f *finder) visit(ctx context.Context, e fsindex.Entry, hit func(*row) error) error {
	if Bookkeeping(e.Name) {
		return fs.SkipDir
	}
	hidden := e.Hidden || ccfs.HiddenByName(e.Name)
	if hidden && !f.p.Hidden {
		return fs.SkipDir
	}
	if _, err := f.fs.Check(f.user, e.Path, ccfs.View); err != nil {
		return fs.SkipDir
	}
	f.scanned++
	r := &row{Entry: e, At: path.Dir(e.Path), tagged: f.tags}
	if !f.cheap(r) {
		return nil
	}
	if len(f.q.Meta) == 0 {
		if !f.q.matchFields(r) {
			return nil
		}
		return f.finish(ctx, r, hit)
	}
	f.batch = append(f.batch, r)
	if len(f.batch) >= metaBatch {
		return f.flush(ctx, hit)
	}
	return nil
}

// cheap runs the tests that need nothing but the walk itself.
func (f *finder) cheap(r *row) bool {
	if !f.q.matchName(r) || !typeOK(r.kind(), f.p.Type) {
		return false
	}
	if !math.IsNaN(f.min) || !math.IsNaN(f.max) {
		size := r.Size
		if r.Dir && f.sizes != nil {
			size = f.sizes(r.Entry)
		}
		if !math.IsNaN(f.min) && float64(size) < f.min {
			return false
		}
		if !math.IsNaN(f.max) && float64(size) > f.max {
			return false
		}
	}
	switch f.p.When {
	case "", "any":
	case "old":
		if !r.MTime.Before(f.now.Add(-365 * day)) {
			return false
		}
	default:
		if r.MTime.Before(f.now.Add(-time.Duration(whenDays[f.p.When]) * day)) {
			return false
		}
	}
	return true
}

func (f *finder) flush(ctx context.Context, hit func(*row) error) error {
	if len(f.batch) == 0 {
		return nil
	}
	files := make([]meta.File, len(f.batch))
	for i, r := range f.batch {
		files[i] = meta.File{Path: r.Path, Real: r.Real, Size: r.Size, MTime: r.MTime, Dir: r.Dir}
	}
	readCtx := ctx
	if !time.Now().Before(f.readUntil) {
		var cancel context.CancelFunc
		readCtx, cancel = context.WithCancel(ctx)
		cancel()
	}
	values, unread := f.meta.Lookup(readCtx, files, f.q.Meta)
	batch := f.batch
	f.batch = nil
	for _, r := range batch {
		r.meta = values[r.Path]
		if !f.q.matchFields(r) {
			if unread[r.Path] {
				f.pending++
			}
			continue
		}
		if err := f.finish(ctx, r, hit); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (f *finder) finish(ctx context.Context, r *row, hit func(*row) error) error {
	if f.extra != nil {
		ok, err := f.extra(ctx, r)
		if err != nil || !ok {
			return err
		}
	}
	return hit(r)
}
