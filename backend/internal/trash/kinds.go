package trash

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// Job kinds of this package.
const (
	KindTrash   = "trash"
	KindRestore = "restore"
	KindEmpty   = "emptyTrash"
)

func init() {
	jobs.RegisterKind(KindTrash, jobs.Kind{Prepare: prepareTrash, Run: runTrash})
	jobs.RegisterKind(KindRestore, jobs.Kind{Run: runRestore})
	jobs.RegisterKind(KindEmpty, jobs.Kind{Run: runEmpty})
	undo.Register(KindTrash, undo.Reverser{Job: undoTrash})
}

// PathsParams are the params of the trash and delete kinds.
type PathsParams struct {
	Paths []string `json:"paths"`
}

func prepareTrash(ctx context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p PathsParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if len(p.Paths) == 0 {
		return nil, v1.Errorf(http.StatusBadRequest, "paths are required")
	}
	var clean []string
	var items []jobs.Item
	seen := make(map[string]bool, len(p.Paths))
	for _, raw := range p.Paths {
		loc, err := d.FS.Entry(u, raw, ccfs.Delete)
		if err != nil {
			return nil, err
		}
		if _, err := BinFor(loc); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(loc.Real); err != nil {
			return nil, err
		}
		if err := d.FS.Carry(ctx, u, loc, ccfs.Delete); err != nil {
			return nil, err
		}
		if seen[loc.Path] {
			continue
		}
		seen[loc.Path] = true
		clean = append(clean, loc.Path)
		items = append(items, jobs.Item{Path: loc.Path, Name: path.Base(loc.Path)})
	}
	return &jobs.Plan{Params: PathsParams{Paths: clean}, Items: items}, nil
}

type undoData struct {
	IDs []string `json:"ids"`
}

func runTrash(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		if err := run.Wait(ctx); err != nil {
			return err
		}
		loc, u, err := run.Entry(it.Path, ccfs.Delete)
		if err == nil {
			err = run.Deps().FS.Carry(ctx, u, loc, ccfs.Delete)
		}
		var t Item
		if err == nil {
			t, err = s.Put(ctx, run.Owner(), loc)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			run.SetRef(it.Seq, t.ID)
			run.Measure(it.Seq, t.Size, t.Files)
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}

	var ids []string
	var first jobs.Item
	for _, it := range run.Items() {
		if it.State == jobs.ItemDone && it.Ref != "" {
			if len(ids) == 0 {
				first = it
			}
			ids = append(ids, it.Ref)
		}
	}
	if l := undo.For(run.Deps()); l != nil && len(ids) > 0 {
		step := undo.Step{Kind: KindTrash, Name: first.Name, Count: len(ids), Path: path.Dir(first.Path)}
		if step, err := l.Record(run.Owner(), step, undoData{IDs: ids}); err == nil {
			run.SetUndo(step.ID)
		}
	}
	return nil
}

func undoTrash(_ context.Context, run *jobs.Run, data json.RawMessage) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	var d undoData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	var errs []error
	for _, id := range d.IDs {
		if _, err := s.restoreFor(run, id); err != nil && !errors.Is(err, ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// restoreFor restores an entry if the job's owner may still create at its old
// place, which can have changed since the entry went to the bin.
func (s *Store) restoreFor(run *jobs.Run, id string) (string, error) {
	it, err := s.Get(id)
	if err != nil {
		return "", err
	}
	if _, _, err := run.Entry(it.Origin, ccfs.Create); err != nil {
		return "", err
	}
	return s.Restore(id)
}

func runRestore(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		if err := run.Wait(ctx); err != nil {
			return err
		}
		_, err := s.restoreFor(run, it.Ref)
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}

func runEmpty(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no trash")
	}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		err := s.Drop(ctx, run, it.Ref)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ErrNotFound) {
			err = nil
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}

// plan turns trash entries into the items of a restore or empty job.
func plan(entries []Item) *jobs.Plan {
	p := &jobs.Plan{Params: struct{}{}}
	for _, e := range entries {
		p.Items = append(p.Items, jobs.Item{Path: e.Origin, Name: e.Name, Ref: e.ID, Bytes: e.Size, Files: e.Files})
	}
	return p
}
