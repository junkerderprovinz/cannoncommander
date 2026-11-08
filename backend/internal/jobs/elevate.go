package jobs

import (
	"context"
	"errors"
	"net/http"
	"path"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

var (
	errNotElevatable = v1.Coded(http.StatusConflict, "notElevatable", "this job cannot go on with administrator rights")
	errDeclined      = v1.Coded(http.StatusForbidden, "elevationDeclined", "administrator rights were not given")
	errNoHelper      = v1.Coded(http.StatusBadGateway, "elevationFailed", "the helper with administrator rights did not answer")
	errUnsupported   = v1.Coded(http.StatusNotImplemented, "elevationUnsupported", "administrator rights cannot be asked for on this system")
)

// Elevate runs the failed and unfinished items of an ended job again with
// administrator rights. Only the desktop app asks for them, since the server
// in a container has no person at its screen to answer.
func (m *Manager) Elevate(id string) (Job, error) {
	if !desktop.Active() {
		return Job{}, errNotElevatable
	}
	return m.retry(id, true)
}

// Elevated has the helper with administrator rights do ops[i] for items[i]
// and finishes each item with what came of it. after runs for every item
// whose operation went through. The items that the helper never got to fail
// with the reason, such as a prompt that was turned down.
func (r *Run) Elevated(ctx context.Context, items []Item, ops []elevate.Op, after func(Item)) error {
	if len(items) == 0 {
		return nil
	}
	for i, it := range items {
		r.Begin(it.Seq)
		if ops[i].From != "" {
			if b, f, err := TreeSize(ctx, ops[i].From); err == nil {
				r.Measure(it.Seq, b, f)
			}
		}
	}
	r.Current(path.Base(items[0].Path))
	done := make([]bool, len(items))
	err := elevate.Run(ctx, ops, func(i int, err error) {
		done[i] = true
		var oe *elevate.OpError
		if errors.As(err, &oe) {
			err = v1.Coded(http.StatusConflict, oe.Code, oe.Msg)
		}
		if err == nil && after != nil {
			after(items[i])
		}
		r.Finish(items[i].Seq, ItemDone, err)
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		switch {
		case errors.Is(err, elevate.ErrDeclined):
			err = errDeclined
		case errors.Is(err, elevate.ErrUnsupported):
			err = errUnsupported
		case errors.Is(err, elevate.ErrNoHelper):
			err = errNoHelper
		}
		for i, it := range items {
			if !done[i] {
				r.Finish(it.Seq, ItemDone, err)
			}
		}
	}
	return nil
}
