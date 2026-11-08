package trash

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

var (
	ErrNoBin     = v1.Coded(http.StatusConflict, "noSystemTrash", "this drive has no trash, delete it for good instead")
	errCancelled = v1.Coded(http.StatusConflict, "trashCancelled", "the delete was stopped")
)

// system reports whether loc goes to the trash of the system. Remotes keep
// the app's own trash, since a cloud has no trash of the computer.
func (s *Store) system(loc ccfs.Location) bool {
	return s.sys != nil && loc.Source != nil && loc.Source.Mount != "/remotes"
}

// systemID tells the ids of the system's trash, which always hold a dot,
// from those of the app's own.
func systemID(id string) bool { return strings.Contains(id, ".") }

func (s *Store) putSystem(ctx context.Context, owner string, loc ccfs.Location) (Item, error) {
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
		return Item{}, ErrShareRoot
	}
	size, files, err := jobs.TreeSize(ctx, loc.Real)
	if err != nil {
		return Item{}, err
	}
	e, err := s.sys.Put(loc.Real)
	if err != nil {
		return Item{}, systemErr(err)
	}
	it := s.itemOf(e)
	it.Origin, it.From, it.Size, it.Files, it.By = loc.Path, path.Dir(loc.Path), size, files, owner
	s.carry(ctx, it.Origin, it.Bin)
	s.changed()
	return it, nil
}

func systemErr(err error) error {
	switch {
	case errors.Is(err, sysbin.ErrNoBin):
		return ErrNoBin
	case errors.Is(err, sysbin.ErrCancelled):
		return errCancelled
	case errors.Is(err, sysbin.ErrNotFound):
		return ErrNotFound
	}
	return err
}

// itemOf describes an entry of the system's trash in API paths where it has
// them. An entry that some other program deleted outside the sources keeps
// its path on disk.
func (s *Store) itemOf(e sysbin.Entry) Item {
	it := Item{ID: e.ID, Name: e.Name, Dir: e.Dir, Size: e.Size, Files: 1, DeletedAt: e.Deleted, native: e.Origin}
	if e.Dir {
		it.Files = 0
	}
	if p, ok := s.apiPath(e.Origin); ok {
		it.Origin, it.From = p, path.Dir(p)
	} else {
		it.Origin, it.From, it.outside = e.Origin, filepath.Dir(e.Origin), true
	}
	if p, ok := s.apiPath(e.Path); ok {
		it.Bin = p
	}
	return it
}

// apiPath finds the source that holds a path on disk.
func (s *Store) apiPath(real string) (string, bool) {
	for _, src := range s.fs.Sources() {
		rel, err := filepath.Rel(src.Root, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return path.Join(src.Mount, filepath.ToSlash(rel)), true
	}
	return "", false
}

// carry tells the panels and the labels that an entry moved, or that it left
// the sources when its new place lies outside them.
func (s *Store) carry(ctx context.Context, from, to string) {
	switch {
	case from != "" && to != "":
		fschange.Moved(from, to)
		s.follow(labels.Moved(ctx, s.deps, labels.Move{From: from, To: to}))
	case from != "":
		fschange.Removed(from)
		s.follow(labels.Removed(ctx, s.deps, from))
	case to != "":
		fschange.Created(to)
	}
}

func (s *Store) getSystem(id string) (Item, error) {
	e, err := s.sys.Get(id)
	if err != nil {
		return Item{}, systemErr(err)
	}
	return s.itemOf(e), nil
}

func (s *Store) listSystem() ([]Item, error) {
	entries, err := s.sys.List()
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(entries))
	for _, e := range entries {
		out = append(out, s.itemOf(e))
	}
	return out, nil
}

// restoreSystem puts an entry back where it came from, as Restore does for
// the app's own trash.
func (s *Store) restoreSystem(id string) (string, error) {
	e, it, err := s.restorable(id)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(e.Origin)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	to := filepath.Join(dir, ccfs.FreeName(dir, e.Name))
	if err := s.sys.Restore(id, to); err != nil {
		return "", systemErr(err)
	}
	return s.restored(it, to), nil
}

// restorable is the entry id of the system's trash, once its old folder is
// known to stay inside its source.
func (s *Store) restorable(id string) (sysbin.Entry, Item, error) {
	e, err := s.sys.Get(id)
	if err != nil {
		return sysbin.Entry{}, Item{}, systemErr(err)
	}
	it := s.itemOf(e)
	if !it.outside {
		parent, err := s.fs.Resolve(it.From)
		if err != nil {
			return e, it, err
		}
		if err := ccfs.Contained(parent); err != nil {
			return e, it, err
		}
	}
	return e, it, nil
}

// restored tells about an entry that came back to the path to and returns
// that path, as the API has it where it can.
func (s *Store) restored(it Item, to string) string {
	back, ok := s.apiPath(to)
	if ok {
		s.carry(context.Background(), it.Bin, back)
	} else {
		back = to
	}
	s.changed()
	return back
}

// restoreOp is the helper's operation that puts an entry of the system's
// trash back with administrator rights, after the checks a normal restore
// makes. It returns where the entry lands.
func (s *Store) restoreOp(run *jobs.Run, id string) (elevate.Op, Item, string, error) {
	if s.sys == nil || !systemID(id) {
		return elevate.Op{}, Item{}, "", errOwnBin
	}
	e, it, err := s.restorable(id)
	if err == nil && it.outside {
		err = run.RequireAdmin()
	} else if err == nil {
		_, _, err = run.Entry(it.Origin, ccfs.Create)
	}
	if err != nil {
		return elevate.Op{}, it, "", err
	}
	dir := filepath.Dir(e.Origin)
	to := filepath.Join(dir, ccfs.FreeName(dir, e.Name))
	return elevate.Op{Kind: elevate.Move, From: e.Path, To: to, Parents: true}, it, to, nil
}

var errOwnBin = v1.Coded(http.StatusConflict, "notElevatable", "this cannot go on with administrator rights")

// elevateRestore puts entries back with administrator rights, for a folder
// that refuses the person.
func elevateRestore(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	var items []jobs.Item
	var ops []elevate.Op
	back := map[int]func(){}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		op, entry, to, err := s.restoreOp(run, it.Ref)
		if err != nil {
			run.Finish(it.Seq, jobs.ItemDone, err)
			continue
		}
		id := it.Ref
		items, ops = append(items, it), append(ops, op)
		back[it.Seq] = func() { s.settle(id, entry, to) }
	}
	return run.Elevated(ctx, items, ops, func(it jobs.Item) { back[it.Seq]() })
}

// settle finishes a restore the helper did: the record goes and the panels
// hear of it.
func (s *Store) settle(id string, it Item, to string) {
	if err := s.sys.Forget(id); err != nil {
		logger.Errorf("trash: %v", err)
	}
	s.restored(it, to)
}

// elevateUndoTrash takes a trash step back with administrator rights.
func elevateUndoTrash(ctx context.Context, run *jobs.Run, data json.RawMessage) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	var d undoData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	var errs []error
	var ops []elevate.Op
	var back []func()
	for _, id := range d.IDs {
		op, entry, to, err := s.restoreOp(run, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ops, back = append(ops, op), append(back, func() { s.settle(id, entry, to) })
	}
	if len(ops) > 0 {
		err := elevate.Run(ctx, ops, func(i int, err error) {
			if err != nil {
				errs = append(errs, jobs.ElevationError(err))
			} else {
				back[i]()
			}
		})
		if err != nil {
			errs = append(errs, jobs.ElevationError(err))
		}
	}
	return errors.Join(errs...)
}

func (s *Store) dropSystem(ctx context.Context, run *jobs.Run, id string) error {
	e, err := s.sys.Get(id)
	if err != nil {
		return systemErr(err)
	}
	if run != nil {
		err = jobs.RemoveTree(ctx, run, e.Path)
	} else {
		err = os.RemoveAll(e.Path)
	}
	if err != nil {
		return err
	}
	if err := s.sys.Forget(id); err != nil {
		return systemErr(err)
	}
	if it := s.itemOf(e); it.Bin != "" {
		s.carry(ctx, it.Bin, "")
	}
	s.changed()
	return nil
}

// mayRestore checks that u may put an entry back. An entry from outside the
// sources only goes back for an admin, who stands for the person at the
// computer.
func (s *Store) mayRestore(u *users.User, it Item) error {
	if it.outside {
		if !u.Permissions.Admin {
			return ccfs.ErrForbidden
		}
		return nil
	}
	_, err := s.fs.Entry(u, it.Origin, ccfs.Create)
	return err
}

// takeOver moves what the app's own trash holds into the system's, keeping
// where and when each entry was deleted, and removes the emptied folders. It
// runs on every start of the desktop app and finds nothing to do once the
// move is through.
func (s *Store) takeOver() {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.query(`ORDER BY deleted_at`)
	if err != nil {
		logger.Errorf("trash: %v", err)
		return
	}
	var bins []string
	for _, it := range items {
		if s.ctx.Err() != nil {
			return
		}
		held, err := s.fs.Resolve(it.Bin)
		if err != nil || !s.system(held) {
			continue
		}
		origin, err := s.fs.Resolve(it.Origin)
		if err != nil {
			continue
		}
		if _, err := s.sys.Adopt(held.Real, origin.Real, it.DeletedAt); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger.Errorf("trash: moving %s into the trash of the system: %v", held.Real, err)
			continue
		}
		s.forget(it.ID)
		os.Remove(filepath.Dir(held.Real))
		bins = append(bins, filepath.Dir(filepath.Dir(held.Real)))
	}
	for _, src := range s.fs.Sources() {
		if src.Mount != "/remotes" {
			bins = append(bins, filepath.Join(src.Root, DirName))
		}
	}
	slices.Sort(bins)
	for _, bin := range slices.Compact(bins) {
		s.adoptLeftovers(bin)
	}
	if len(items) > 0 {
		s.changed()
	}
}

// adoptLeftovers takes over what lies in an old trash folder without a
// record, as if it had been deleted from the folder that holds the trash.
func (s *Store) adoptLeftovers(bin string) {
	holders, err := os.ReadDir(bin)
	if err != nil {
		return
	}
	top := filepath.Dir(bin)
	for _, h := range holders {
		dir := filepath.Join(bin, h.Name())
		at := time.Now()
		if info, err := h.Info(); err == nil {
			at = info.ModTime()
		}
		inner, _ := os.ReadDir(dir)
		for _, e := range inner {
			if _, err := s.sys.Adopt(filepath.Join(dir, e.Name()), filepath.Join(top, e.Name()), at); err != nil {
				logger.Errorf("trash: moving %s into the trash of the system: %v", filepath.Join(dir, e.Name()), err)
			}
		}
		os.Remove(dir)
	}
	if os.Remove(bin) == nil {
		logger.Infof("trash: moved %s into the trash of the system", bin)
	}
}

// elevateTrash puts the open items in the Recycle Bin with administrator
// rights. Where the helper cannot recycle, on Linux, where root has no trash
// of the person, or on a drive without one, the items fail as having no
// trash, which offers to delete them for good.
func elevateTrash(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	var items []jobs.Item
	var ops []elevate.Op
	origins := map[int]string{}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		loc, u, err := run.Entry(it.Path, ccfs.Delete)
		if err == nil {
			err = run.Deps().FS.Carry(ctx, u, loc, ccfs.Delete)
		}
		if err == nil && (!s.system(loc) || !elevate.Recycles()) {
			err = ErrNoBin
		}
		if err != nil {
			run.Finish(it.Seq, jobs.ItemDone, err)
			continue
		}
		items, ops, origins[it.Seq] = append(items, it), append(ops, elevate.Op{Kind: elevate.Recycle, From: loc.Real}), loc.Real
	}
	since := time.Now().Add(-2 * time.Second)
	err := run.Elevated(ctx, items, ops, func(it jobs.Item) {
		if found, ok := s.recycled(origins[it.Seq], since); ok {
			run.SetRef(it.Seq, found.ID)
			s.carry(ctx, it.Path, found.Bin)
		} else {
			s.carry(ctx, it.Path, "")
		}
		s.changed()
	})
	recordUndo(run)
	return err
}

// recycled finds the entry that the helper put in the trash from origin.
func (s *Store) recycled(origin string, since time.Time) (Item, bool) {
	entries, err := s.sys.List()
	if err != nil {
		return Item{}, false
	}
	var found sysbin.Entry
	for _, e := range entries {
		if strings.EqualFold(e.Origin, origin) && !e.Deleted.Before(since) && e.Deleted.After(found.Deleted) {
			found = e
		}
	}
	if found.ID == "" {
		return Item{}, false
	}
	return s.itemOf(found), true
}
