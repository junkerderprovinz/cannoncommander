//go:build desktop

package jobs

import (
	"context"
	"errors"
	"net/http"
	"path"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

var (
	errDeclined    = v1.Coded(http.StatusForbidden, "elevationDeclined", "administrator rights were not given")
	errNoHelper    = v1.Coded(http.StatusBadGateway, "elevationFailed", "the helper with administrator rights did not answer")
	errUnsupported = v1.Coded(http.StatusNotImplemented, "elevationUnsupported", "administrator rights cannot be asked for on this system")
)

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
		err = ElevationError(err)
		if err == nil && after != nil {
			after(items[i])
		}
		r.Finish(items[i].Seq, ItemDone, err)
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		err = ElevationError(err)
		for i, it := range items {
			if !done[i] {
				r.Finish(it.Seq, ItemDone, err)
			}
		}
	}
	return nil
}

// ElevationError says an error of elevate.Run, or of one of its operations,
// the way the app's problems do, so the interface can word it.
func ElevationError(err error) error {
	var oe *elevate.OpError
	switch {
	case errors.As(err, &oe):
		return v1.Coded(http.StatusConflict, oe.Code, oe.Msg)
	case errors.Is(err, elevate.ErrDeclined):
		return errDeclined
	case errors.Is(err, elevate.ErrUnsupported):
		return errUnsupported
	case errors.Is(err, elevate.ErrNoHelper):
		return errNoHelper
	}
	return err
}
