//go:build desktop

package trash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/plainpath"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

var (
	ErrNoBin     = v1.Coded(http.StatusConflict, "noSystemTrash", "this drive has no trash, delete it for good instead")
	errCancelled = v1.Coded(http.StatusConflict, "trashCancelled", "the delete was stopped")
)

func init() {
	jobs.Elevates(KindTrash, elevateTrash)
	jobs.Elevates(KindRestore, elevateRestore)
	undo.Elevates(KindTrash, elevateUndoTrash)
}

// osBin is the trash of the operating system as the store uses it.
type osBin struct {
	*Store
	bin sysbin.Bin
}

// systemTrash opens the trash of the system in the desktop app. Without one
// the app keeps its own.
func systemTrash(s *Store) systemBin {
	if !desktop.Active() {
		return nil
	}
	bin, err := sysbin.Open()
	if err != nil {
		logger.Warningf("trash: %v, the app keeps its own", err)
		return nil
	}
	return &osBin{Store: s, bin: bin}
}

// takes reports whether loc goes to the trash of the system. Remotes keep
// the app's own trash, since a cloud has no trash of the computer.
func (s *osBin) takes(loc ccfs.Location) bool {
	return loc.Source != nil && loc.Source.Mount != "/remotes"
}

func (s *osBin) put(ctx context.Context, owner string, loc ccfs.Location) (Item, error) {
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
		return Item{}, ErrShareRoot
	}
	size, files, err := jobs.TreeSize(ctx, loc.Real)
	if err != nil {
		return Item{}, err
	}
	e, err := s.bin.Put(loc.Real)
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
	it := Item{ID: e.ID, Name: e.Name, Dir: e.Dir, Size: e.Size, Files: 1, DeletedAt: e.Deleted}
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

func (s *osBin) get(id string) (Item, error) {
	e, err := s.bin.Get(id)
	if err != nil {
		return Item{}, systemErr(err)
	}
	return s.itemOf(e), nil
}

func (s *osBin) list() ([]Item, error) {
	entries, err := s.bin.List()
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(entries))
	for _, e := range entries {
		out = append(out, s.itemOf(e))
	}
	return out, nil
}

// restore puts an entry back where it came from, as Restore does for the
// app's own trash.
func (s *osBin) restore(id string) (string, error) {
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
	if err := s.bin.Restore(id, to); err != nil {
		return "", systemErr(err)
	}
	return s.restored(it, to), nil
}

// restorable is the entry id of the system's trash, once its old folder is
// known to stay inside its source.
func (s *osBin) restorable(id string) (sysbin.Entry, Item, error) {
	e, err := s.bin.Get(id)
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
func (s *osBin) restoreOp(run *jobs.Run, id string) (elevate.Op, Item, string, error) {
	if !systemID(id) {
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
	s, err := osBinOf(run.Deps())
	if err != nil {
		return err
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
func (s *osBin) settle(id string, it Item, to string) {
	if err := s.bin.Forget(id); err != nil {
		logger.Errorf("trash: %v", err)
	}
	s.restored(it, to)
}

// elevateUndoTrash takes a trash step back with administrator rights.
func elevateUndoTrash(ctx context.Context, run *jobs.Run, data json.RawMessage) error {
	s, err := osBinOf(run.Deps())
	if err != nil {
		return err
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

func (s *osBin) drop(ctx context.Context, run *jobs.Run, id string) error {
	e, err := s.bin.Get(id)
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
	if err := s.bin.Forget(id); err != nil {
		return systemErr(err)
	}
	if it := s.itemOf(e); it.Bin != "" {
		s.carry(ctx, it.Bin, "")
	}
	s.changed()
	return nil
}

// takeOver moves the entries the app's own trash recorded into the system's,
// keeping where and when each was deleted, and removes the emptied folders.
// It runs on every start of the desktop app and finds nothing to do once the
// move is through.
//
// The trash folder sits at the top of a drive, where other accounts may make
// folders too, and so may whoever made a USB disk. Only records count, and a
// trash folder, holder or entry that is a link or belongs to someone else
// stays where it is, so nobody can steer the move at somebody's profile.
func (s *osBin) takeOver() {
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
		if err != nil || !s.takes(held) {
			continue
		}
		origin, err := s.fs.Resolve(it.Origin)
		if err != nil {
			continue
		}
		holder := filepath.Dir(held.Real)
		bin := filepath.Dir(holder)
		if err := ownPlain(bin, holder); err != nil {
			logger.Warningf("trash: leaving %s where it is: %v", held.Real, err)
			continue
		}
		if err := plainpath.Check(held.Real); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger.Warningf("trash: leaving %s where it is: %v", held.Real, err)
			continue
		}
		if _, err := s.bin.Adopt(held.Real, origin.Real, it.DeletedAt); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger.Errorf("trash: moving %s into the trash of the system: %v", held.Real, err)
			continue
		}
		s.forget(it.ID)
		os.Remove(holder)
		bins = append(bins, bin)
	}
	slices.Sort(bins)
	for _, bin := range slices.Compact(bins) {
		if ownPlain(bin) == nil && os.Remove(bin) == nil {
			logger.Infof("trash: moved %s into the trash of the system", bin)
		}
	}
	if len(items) > 0 {
		s.changed()
	}
}

// ownPlain checks that every one of dirs is a plain folder of this account.
func ownPlain(dirs ...string) error {
	for _, d := range dirs {
		if err := plainpath.Check(d); err != nil {
			return err
		}
		mine, err := plainpath.Mine(d)
		if err != nil {
			return err
		}
		if !mine {
			return fmt.Errorf("%s belongs to another account", d)
		}
	}
	return nil
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
	b, _ := s.sys.(*osBin)
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
		if err == nil && (b == nil || !b.takes(loc) || !elevate.Recycles()) {
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
		if found, ok := b.recycled(origins[it.Seq], since); ok {
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
func (s *osBin) recycled(origin string, since time.Time) (Item, bool) {
	entries, err := s.bin.List()
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

// osBinOf is the trash of the system for a job with administrator rights. A
// store that keeps its own has nothing that such a job could go on with.
func osBinOf(d v1.Deps) (*osBin, error) {
	s := For(d)
	if s == nil {
		return nil, errors.New("no trash")
	}
	b, ok := s.sys.(*osBin)
	if !ok {
		return nil, errOwnBin
	}
	return b, nil
}
