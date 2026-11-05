package fileops

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
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

func init() {
	undo.Register(StepRename, undo.Reverser{Inline: undoRename})
	undo.Register(StepMkdir, undo.Reverser{Inline: undoMkdir})
	undo.Register(StepTouch, undo.Reverser{Inline: undoTouch})
	undo.Register(KindCopy, undo.Reverser{Job: undoCopy})
	undo.Register(KindMove, undo.Reverser{Job: undoMove})
}

type renameData struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type entryData struct {
	Path  string    `json:"path"`
	MTime time.Time `json:"mtime,omitzero"`
}

var errChanged = v1.Errorf(http.StatusConflict, "it changed since, so it is left as it is")

func undoRename(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) error {
	var data renameData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	s := For(d)
	to, err := s.existing(u, data.To, ccfs.Modify)
	if err != nil {
		return err
	}
	if err := s.fs.Carry(context.Background(), u, to, ccfs.Modify); err != nil {
		return err
	}
	from, err := s.newEntry(u, data.From)
	if errors.Is(err, errExists) {
		return v1.Errorf(http.StatusConflict, "the old name is taken again")
	}
	if err != nil {
		return err
	}
	if err := renameNoReplace(to.Real, from.Real); err != nil {
		return err
	}
	fschange.Moved(to.Path, from.Path)
	For(d).moved(context.Background(), labels.Move{From: to.Path, To: from.Path})
	return nil
}

func undoMkdir(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) error {
	var data entryData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	loc, err := For(d).existing(u, data.Path, ccfs.Delete)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(loc.Real)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return v1.Errorf(http.StatusConflict, "the folder is not empty anymore")
	}
	if err := os.Remove(loc.Real); err != nil {
		return err
	}
	fschange.Removed(loc.Path)
	return nil
}

func undoTouch(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) error {
	var data entryData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	loc, err := For(d).existing(u, data.Path, ccfs.Delete)
	if err != nil {
		return err
	}
	info, err := os.Lstat(loc.Real)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != 0 || !info.ModTime().Equal(data.MTime) {
		return errChanged
	}
	if err := os.Remove(loc.Real); err != nil {
		return err
	}
	fschange.Removed(loc.Path)
	return nil
}

// undoCopy deletes what the copy created and brings back from the trash
// what it replaced.
func undoCopy(ctx context.Context, run *jobs.Run, raw json.RawMessage) error {
	var data transferUndo
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	s := For(run.Deps())
	var errs []error
	for _, p := range slices.Backward(data.Created) {
		loc, u, err := run.Entry(p, ccfs.Delete)
		if err == nil {
			err = s.fs.Carry(ctx, u, loc, ccfs.Delete)
		}
		if err == nil {
			err = jobs.RemoveTree(ctx, run, loc.Real)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fschange.Removed(p)
		s.removed(ctx, p)
	}
	return errors.Join(append(errs, restoreReplaced(run, s, data.Replaced))...)
}

// undoMove moves everything back where it came from, newest first, and
// then brings back what the move replaced.
func undoMove(ctx context.Context, run *jobs.Run, raw json.RawMessage) error {
	var data transferUndo
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	s := For(run.Deps())
	t := &transfer{s: s, run: run, move: true, keep: true, srcPerm: srcPermFor(true), dstPerm: ccfs.Create}
	var errs []error
	for _, m := range slices.Backward(data.Moved) {
		to, u, err := run.EntryCarried(m.To, t.srcPerm)
		if err == nil {
			t.u = u
			err = s.fs.Carry(ctx, u, to, t.srcPerm)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		from, err := s.fs.EntryCarried(u, m.From, t.dstPerm)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(from.Real), 0o777)
		}
		if err == nil {
			_, err = t.entry(ctx, node{to.Path, to.Real}, node{from.Path, from.Real}, path.Base(from.Path), false)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	s.moved(ctx, t.follow...)
	if data.Folder != "" {
		errs = append(errs, removeMadeFolder(run, data.Folder))
	}
	return errors.Join(append(errs, restoreReplaced(run, s, data.Replaced))...)
}

// removeMadeFolder leaves the folder alone when something else moved in since.
func removeMadeFolder(run *jobs.Run, p string) error {
	loc, _, err := run.Entry(p, ccfs.Delete)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(loc.Real)
	if errors.Is(err, fs.ErrNotExist) || err == nil && len(entries) > 0 {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(loc.Real); err != nil {
		return err
	}
	fschange.Removed(loc.Path)
	return nil
}

func restoreReplaced(run *jobs.Run, s *Service, replaced []replacedEntry) error {
	if s.trash == nil {
		return nil
	}
	var errs []error
	for _, r := range replaced {
		if _, _, err := run.Reach(r.Path, ccfs.Create); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := s.trash.Restore(r.TrashID); err != nil && !errors.Is(err, trash.ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
