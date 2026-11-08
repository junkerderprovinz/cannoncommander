package jobs

import (
	"context"
	"net/http"
	"sync"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

var errNotElevatable = v1.Coded(http.StatusConflict, "notElevatable", "this job cannot go on with administrator rights")

var (
	elevationsMu sync.RWMutex
	elevations   = map[string]func(ctx context.Context, run *Run) error{}
)

// Elevates lets a kind do its open items again with administrator rights,
// once the job failed for lack of them. Only the desktop build registers
// such steps, and a kind without one cannot go on that way.
func Elevates(kind string, step func(ctx context.Context, run *Run) error) {
	elevationsMu.Lock()
	defer elevationsMu.Unlock()
	elevations[kind] = step
}

func elevation(kind string) func(ctx context.Context, run *Run) error {
	elevationsMu.RLock()
	defer elevationsMu.RUnlock()
	return elevations[kind]
}

// Elevate runs the failed and unfinished items of an ended job again with
// administrator rights. Only the desktop app asks for them, since the server
// in a container has no person at its screen to answer.
func (m *Manager) Elevate(id string) (Job, error) {
	if !desktop.Active() {
		return Job{}, errNotElevatable
	}
	return m.retry(id, true)
}
