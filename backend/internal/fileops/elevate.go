package fileops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

// elevateDelete deletes the open items with administrator rights. The rights
// in the app are checked here as for any delete; the helper only gets the
// paths that passed.
func elevateDelete(ctx context.Context, run *jobs.Run) error {
	s := For(run.Deps())
	var items []jobs.Item
	var ops []elevate.Op
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		loc, u, err := run.Entry(it.Path, ccfs.Delete)
		if err == nil {
			err = run.Deps().FS.Carry(ctx, u, loc, ccfs.Delete)
		}
		if err != nil {
			run.Finish(it.Seq, jobs.ItemDone, err)
			continue
		}
		items = append(items, it)
		ops = append(ops, elevate.Op{Kind: elevate.Delete, From: loc.Real})
	}
	return run.Elevated(ctx, items, ops, func(it jobs.Item) {
		fschange.Removed(it.Path)
		if s != nil {
			s.removed(ctx, it.Path)
		}
	})
}

// elevateTransfer copies or moves the open items with administrator rights.
// A name taken at the target is decided once for the whole item, as the
// helper cannot ask while it works. Nothing done this way can be taken back,
// since that would need the rights again.
func elevateTransfer(move bool) func(context.Context, *jobs.Run) error {
	return func(ctx context.Context, run *jobs.Run) error {
		s := For(run.Deps())
		var p TransferParams
		if err := run.Params(&p); err != nil {
			return err
		}
		srcPerm, dstPerm := srcPermFor(move), destPermFor(p.Rule)
		dest, _, err := run.Reach(p.Dest, dstPerm)
		if err != nil {
			return err
		}
		// The folder of a move into a new folder may be what was refused, and
		// the helper makes it on the way.
		_, missing := os.Lstat(dest.Real)
		fresh := p.NewFolder && errors.Is(missing, fs.ErrNotExist)
		if !fresh {
			if err := folder(dest); err != nil {
				return err
			}
		}
		kind := elevate.Copy
		if move {
			kind = elevate.Move
		}
		var items []jobs.Item
		var ops []elevate.Op
		targets := map[int]string{}
		for _, it := range run.Items() {
			if it.State != jobs.ItemPending {
				continue
			}
			op, target, skip, err := planElevated(ctx, run, it, dest, kind, srcPerm, dstPerm)
			op.Parents = fresh
			if ctx.Err() != nil {
				return ctx.Err()
			}
			switch {
			case err != nil:
				run.Finish(it.Seq, jobs.ItemDone, err)
			case skip:
				run.Finish(it.Seq, jobs.ItemSkipped, nil)
			default:
				items, ops, targets[it.Seq] = append(items, it), append(ops, op), target
			}
		}
		return run.Elevated(ctx, items, ops, func(it jobs.Item) {
			to := targets[it.Seq]
			if s == nil {
				return
			}
			if move {
				fschange.Moved(it.Path, to)
				s.moved(ctx, labels.Move{From: it.Path, To: to})
			} else {
				fschange.Created(to)
				s.copied(ctx, labels.Move{From: it.Path, To: to})
			}
		})
	}
}

// planElevated turns one item into the helper's operation, after the same
// checks of the app's rights as a normal run, and settles a name taken at
// the target by the job's answers or by asking.
func planElevated(ctx context.Context, run *jobs.Run, it jobs.Item, dest ccfs.Location, kind string, srcPerm, dstPerm ccfs.Perm) (op elevate.Op, target string, skip bool, err error) {
	src, u, err := run.Entry(it.Path, srcPerm)
	if err != nil {
		return op, "", false, err
	}
	if kind == elevate.Move {
		if err := run.Deps().FS.Carry(ctx, u, src, srcPerm); err != nil {
			return op, "", false, err
		}
	}
	name := it.Name
	if name == "" {
		name = path.Base(src.Path)
	}
	target = path.Join(dest.Path, name)
	to, _, err := run.Entry(target, dstPerm)
	if err != nil {
		return op, "", false, err
	}
	op = elevate.Op{Kind: kind, From: src.Real, To: to.Real}
	if op.Skip, err = leftBehind(ctx, run.Deps().FS, u, src, srcPerm, kind == elevate.Move); err != nil {
		return op, "", false, err
	}
	there, err := os.Lstat(to.Real)
	if err != nil {
		return op, target, false, nil
	}
	info, err := os.Lstat(src.Real)
	if err != nil {
		return op, "", false, err
	}
	choice := run.Answer(name)
	if choice == "" {
		c := jobs.NewConflict(name, side(node{src.Path, src.Real}, info), side(node{target, to.Real}, there))
		if choice, err = run.Ask(ctx, c); err != nil {
			return op, "", false, err
		}
	}
	switch choice {
	case jobs.Skip:
		return op, "", true, nil
	case jobs.Newer:
		if !newer(info, there) {
			return op, "", true, nil
		}
		op.Replace = true
	case jobs.Replace:
		op.Replace = true
	case jobs.Both:
		free := ccfs.FreeName(dest.Real, name)
		target, op.To = path.Join(dest.Path, free), filepath.Join(dest.Real, free)
	}
	return op, target, false, nil
}

// leftBehind lists what a normal copy or move of src leaves where it is: part
// files, and what the owner may not see or, for a move, may not take along.
func leftBehind(ctx context.Context, fsys *ccfs.Resolver, u *users.User, src ccfs.Location, perm ccfs.Perm, move bool) ([]string, error) {
	check := fsys.Check
	if move {
		check = fsys.CheckCarried
	}
	var out []string
	err := filepath.WalkDir(src.Real, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == src.Real {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(src.Real, p)
		if err != nil {
			return err
		}
		if _, err := check(u, path.Join(src.Path, filepath.ToSlash(rel)), perm); err != nil || IsPart(d.Name()) {
			out = append(out, p)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return out, err
}
