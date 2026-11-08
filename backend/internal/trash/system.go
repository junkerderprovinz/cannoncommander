package trash

import (
	"context"
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
	e, err := s.sys.Get(id)
	if err != nil {
		return "", systemErr(err)
	}
	it := s.itemOf(e)
	if !it.outside {
		parent, err := s.fs.Resolve(it.From)
		if err != nil {
			return "", err
		}
		if err := ccfs.Contained(parent); err != nil {
			return "", err
		}
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
	back, ok := s.apiPath(to)
	if ok {
		s.carry(context.Background(), it.Bin, back)
	} else {
		back = to
	}
	s.changed()
	return back, nil
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
