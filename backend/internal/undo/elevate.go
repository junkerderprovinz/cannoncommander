//go:build desktop

package undo

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

func init() {
	jobs.Elevates(JobKind, elevateUndo)
}

var errNotElevatable = v1.Coded(http.StatusConflict, "notElevatable", "this cannot go on with administrator rights")

var (
	elevationsMu sync.RWMutex
	elevations   = map[string]takeBack{}
)

// Elevates lets a kind of step be taken back with administrator rights, once
// its reverser's Job failed for lack of them.
func Elevates(kind string, f func(ctx context.Context, run *jobs.Run, data json.RawMessage) error) {
	elevationsMu.Lock()
	defer elevationsMu.Unlock()
	elevations[kind] = f
}

func elevateUndo(ctx context.Context, run *jobs.Run) error {
	return undoWith(ctx, run, func(kind string) (takeBack, error) {
		elevationsMu.RLock()
		defer elevationsMu.RUnlock()
		if do := elevations[kind]; do != nil {
			return do, nil
		}
		return nil, errNotElevatable
	})
}
