package labels

import (
	"context"
	"path"
	"slices"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// Moved carries the labels of each From over to its To. Call it once the
// rename or move has succeeded, for a whole job at once if it is handy.
func Moved(ctx context.Context, d v1.Deps, moves ...Move) error {
	return transferAll(ctx, d, moves, false)
}

// Copied gives each To the labels of its From, which keeps its own.
func Copied(ctx context.Context, d v1.Deps, copies ...Move) error {
	return transferAll(ctx, d, copies, true)
}

// Removed drops the labels of the given paths. Call it after a delete
// that cannot be undone. Moving to the trash is a move.
func Removed(ctx context.Context, d v1.Deps, paths ...string) error {
	clean, err := cleanAll(paths)
	if err != nil {
		return err
	}
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var touched []string
	for _, p := range clean {
		n, err := dropTree(ctx, tx, p)
		if err != nil {
			return err
		}
		if n > 0 {
			touched = append(touched, path.Dir(p))
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	publishItems(d.Hub, touched)
	return nil
}

func transferAll(ctx context.Context, d v1.Deps, moves []Move, keep bool) error {
	clean := make([]Move, 0, len(moves))
	for _, m := range moves {
		from, err := ccfs.Clean(m.From)
		if err != nil {
			return err
		}
		to, err := ccfs.Clean(m.To)
		if err != nil {
			return err
		}
		if from != to {
			clean = append(clean, Move{from, to})
		}
	}
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	touched, err := transfer(ctx, tx, clean, keep)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	publishItems(d.Hub, touched)
	return nil
}

func cleanAll(paths []string) ([]string, error) {
	out := make([]string, len(paths))
	for i, p := range paths {
		c, err := ccfs.Clean(p)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// publishItems tells the clients watching each folder that labels of its
// entries changed. Only watchers of a folder hear about it, because they
// already passed the check that they may see it.
func publishItems(h *hub.Hub, folders []string) {
	slices.Sort(folders)
	for _, f := range slices.Compact(folders) {
		_, _ = h.PublishJSON(EventChanged, hub.FolderTopic(f), Change{Kind: "items", Path: f})
	}
}
