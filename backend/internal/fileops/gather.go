package fileops

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

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

func init() {
	for _, mode := range []string{unraid.ModeGather, unraid.ModeScatter} {
		jobs.RegisterKind(mode, jobs.Kind{Prepare: prepareGather(mode), Run: runGather, Heavy: true})
	}
}

// unraidService is swapped in tests for a server laid out in a temp dir.
var unraidService = unraid.Default

type gatherParams struct {
	Mode string `json:"mode"`
	Path string `json:"path"`
	// Disk is the target of a gather and the disk a scatter empties, as
	// chosen from the plan.
	Disk string `json:"disk"`
}

// GatherResult is what a gather or scatter leaves behind. Linked files and
// files the target already has stay where they are.
type GatherResult struct {
	Moved      int    `json:"moved"`
	Linked     int    `json:"linked,omitempty"`
	Exists     int    `json:"exists,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	FirstError string `json:"firstError,omitempty"`
	Partial    bool   `json:"partial,omitempty"`
}

func prepareGather(mode string) func(context.Context, v1.Deps, *users.User, json.RawMessage) (*jobs.Plan, error) {
	return func(_ context.Context, _ v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
		var p gatherParams
		if err := jobs.Decode(raw, &p); err != nil {
			return nil, err
		}
		if !u.Permissions.Admin {
			return nil, v1.Errorf(http.StatusForbidden, "only admins move files between disks")
		}
		p.Mode, p.Path = mode, path.Clean(p.Path)
		if !strings.HasPrefix(p.Path, "/mnt/user/") {
			return nil, v1.Errorf(http.StatusBadRequest, "pick a folder in a user share")
		}
		st, err := unraidService().State()
		if err != nil {
			return nil, err
		}
		if p.Disk == "" || !slices.Contains(st.DataDisks(), p.Disk) {
			return nil, v1.Errorf(http.StatusBadRequest, "pick the disk from the plan first")
		}
		return &jobs.Plan{Params: p, Dest: "/mnt/" + p.Disk, Items: []jobs.Item{{Path: p.Path}}}, nil
	}
}

// runGather plans again at the start, so a run that was cut off only
// moves what is still in the wrong place.
func runGather(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	if s == nil {
		return errors.New("no file service")
	}
	var p gatherParams
	if err := run.Params(&p); err != nil {
		return err
	}
	svc := unraidService()
	plan, err := svc.PlanGather(ctx, unraid.GatherRequest{Path: p.Path, Mode: p.Mode, Disk: p.Disk})
	if err != nil {
		return err
	}
	it := run.Items()[0]
	run.Measure(it.Seq, plan.Move, int64(plan.Moves))
	run.Begin(it.Seq)

	t := &transfer{s: s, run: run, move: true, keep: true, hash: true, resumed: it.Resumed}
	res := GatherResult{Partial: plan.Partial}
	emptied := map[string]bool{}
	for _, m := range plan.MoveList() {
		if isPart(path.Base(m.Rel)) {
			continue
		}
		if m.Linked {
			res.Linked++
			continue
		}
		run.Current(m.Rel)
		err := moveAcross(ctx, t, svc, m, &res)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			res.Failed++
			if res.FirstError == "" {
				res.FirstError = err.Error()
			}
			continue
		}
		emptied[m.From] = true
	}
	folder := strings.TrimPrefix(p.Path, "/mnt/user/")
	for dev := range emptied {
		pruneEmpty(svc.DiskPath(dev, folder), strings.Contains(folder, "/"))
	}
	run.SetResult(res)
	if res.Failed > 0 {
		err = fmt.Errorf("%d files could not be moved: %s", res.Failed, res.FirstError)
	}
	run.Finish(it.Seq, jobs.ItemDone, err)
	return nil
}

// moveAcross copies one file to the other disk, checks it and removes the
// original. A target that is the finished copy of an earlier run only
// needs the original removed.
func moveAcross(ctx context.Context, t *transfer, svc *unraid.Service, m unraid.Move, res *GatherResult) error {
	src := node{"/mnt/" + m.From + "/" + m.Rel, svc.DiskPath(m.From, m.Rel)}
	dst := node{"/mnt/" + m.To + "/" + m.Rel, svc.DiskPath(m.To, m.Rel)}
	info, err := os.Lstat(src.real)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", m.Rel)
	}
	if target, err := os.Lstat(dst.real); err == nil {
		if !t.resumed || !ours(info, target) {
			res.Exists++
			return nil
		}
		if err := os.Remove(src.real); err != nil {
			return err
		}
		res.Moved++
		return t.run.Advance(ctx, info.Size(), 1)
	}
	if err := mkdirLike(svc.DiskPath(m.From, ""), svc.DiskPath(m.To, ""), path.Dir(m.Rel)); err != nil {
		return err
	}
	if err := t.file(ctx, src, dst, info, false, true); err != nil {
		return err
	}
	fschange.Removed(src.api)
	res.Moved++
	return nil
}

// mkdirLike creates the folders of rel below to that are missing, with the
// mode and owner they have below from. Unraid shares rely on both.
func mkdirLike(from, to, rel string) error {
	if rel == "." {
		return nil
	}
	cur := ""
	for _, part := range strings.Split(rel, "/") {
		cur = path.Join(cur, part)
		dst := filepath.Join(to, filepath.FromSlash(cur))
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		info, err := os.Stat(filepath.Join(from, filepath.FromSlash(cur)))
		if err != nil {
			return err
		}
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		_ = os.Chmod(dst, info.Mode().Perm())
		chown(dst, info)
	}
	return nil
}

// pruneEmpty removes the folders below root that the moves left empty,
// deepest first, and root itself when it is not the top of the share.
func pruneEmpty(root string, withRoot bool) {
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && (withRoot || p != root) {
			dirs = append(dirs, p)
		}
		return nil
	})
	for _, d := range slices.Backward(dirs) {
		_ = os.Remove(d)
	}
}
