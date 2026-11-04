package fileops

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"slices"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

func init() {
	jobs.RegisterKind(KindDelete, jobs.Kind{Prepare: prepareDelete, Run: runDelete})
}

func prepareDelete(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p trash.PathsParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if len(p.Paths) == 0 {
		return nil, v1.Errorf(http.StatusBadRequest, "paths are required")
	}
	var clean []string
	var items []jobs.Item
	for _, raw := range p.Paths {
		loc, err := d.FS.Check(u, raw, ccfs.Delete)
		if err != nil {
			return nil, err
		}
		if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
			return nil, v1.Errorf(http.StatusBadRequest, "only files and folders inside a source can be deleted")
		}
		if err := ccfs.Contained(loc); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(loc.Real); err != nil {
			return nil, err
		}
		if slices.Contains(clean, loc.Path) {
			continue
		}
		clean = append(clean, loc.Path)
		items = append(items, jobs.Item{Path: loc.Path, Name: path.Base(loc.Path)})
	}
	return &jobs.Plan{Params: trash.PathsParams{Paths: clean}, Items: items}, nil
}

func runDelete(ctx context.Context, run *jobs.Run) error {
	fsys := run.Deps().FS
	if fsys == nil {
		return errors.New("no file service")
	}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		loc, err := fsys.Resolve(it.Path)
		if err == nil {
			err = ccfs.Contained(loc)
		}
		if err == nil {
			var b, f int64
			if b, f, err = jobs.TreeSize(ctx, loc.Real); err == nil {
				run.Measure(it.Seq, b, f)
				err = jobs.RemoveTree(ctx, run, loc.Real)
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		if err == nil {
			fschange.Removed(it.Path)
			if s := For(run.Deps()); s != nil {
				s.removed(ctx, it.Path)
			}
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}
