package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Run is what a kind sees of its job while it works.
type Run struct {
	m *Manager
	j *job
}

// ID returns the job id.
func (r *Run) ID() string { return r.j.ID }

// Owner returns the user who started the job.
func (r *Run) Owner() string {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.j.Owner
}

// User returns the owner as they are now. A job can wait a long time for its
// turn, for an answer or across a restart, and what its owner could do when it
// started says little by then.
func (r *Run) User() (*users.User, error) {
	if r.m.deps.Users == nil {
		return nil, ccfs.ErrForbidden
	}
	return r.m.deps.Users(r.Owner())
}

// RequireAdmin fails for a job only admins may run when its owner is no admin
// anymore.
func (r *Run) RequireAdmin() error {
	u, err := r.User()
	if err != nil {
		return err
	}
	if !u.Permissions.Admin {
		return ccfs.ErrForbidden
	}
	return nil
}

// Reach is ccfs.Reach for the owner as they are now.
func (r *Run) Reach(p string, perm ccfs.Perm) (ccfs.Location, *users.User, error) {
	u, err := r.User()
	if err != nil {
		return ccfs.Location{}, nil, err
	}
	loc, err := r.m.deps.FS.Reach(u, p, perm)
	return loc, u, err
}

// Deps returns the shared services.
func (r *Run) Deps() v1.Deps { return r.m.deps }

// Params decodes the stored params into v.
func (r *Run) Params(v any) error { return json.Unmarshal(r.j.params, v) }

// Items returns a copy of the job's items.
func (r *Run) Items() []Item {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	out := make([]Item, len(r.j.items))
	copy(out, r.j.items)
	return out
}

// Cancelled reports whether a user cancelled the job, as opposed to the
// server shutting down. Only then should a kind clean up half done work.
func (r *Run) Cancelled() bool {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.j.cancelled
}

// Measure records how big an item is, so the job can show its progress.
func (r *Run) Measure(seq int, bytes, files int64) {
	r.item(seq, func(it *Item) { it.Bytes, it.Files = bytes, files })
	r.m.mu.Lock()
	r.m.schedule()
	r.m.mu.Unlock()
}

// Begin marks an item as the one in work.
func (r *Run) Begin(seq int) {
	r.item(seq, func(it *Item) { it.State = ItemRunning })
	r.m.mu.Lock()
	r.j.inflightBytes, r.j.inflightFiles = 0, 0
	r.m.mu.Unlock()
}

// Finish records the outcome of an item. A nil err with state done or
// skipped counts the whole item as handled.
func (r *Run) Finish(seq int, state string, err error) {
	r.item(seq, func(it *Item) {
		it.State, it.Error, it.Resumed = state, "", false
		if err != nil {
			it.State, it.Error = ItemFailed, errorText(err)
		}
	})
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.j.inflightBytes, r.j.inflightFiles = 0, 0
	r.j.addUp()
	if time.Since(r.j.publishedAt) >= progressEvery {
		r.m.publishProgress(r.j)
	} else {
		r.m.reportSoon(r.j)
	}
}

// SetRef keeps something about an item that the kind needs again after a
// restart, such as the trash entry it became.
func (r *Run) SetRef(seq int, ref string) {
	r.item(seq, func(it *Item) { it.Ref = ref })
}

// item changes one item, which sits at the index of its Seq, and keeps the
// totals of the job in step without going through all its items.
func (r *Run) item(seq int, f func(*Item)) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	it := &r.j.items[seq]
	r.j.tally(it, -1)
	f(it)
	r.j.tally(it, 1)
	r.j.addUp()
	r.m.saveItem(r.j, *it)
}

// Current names the file in work, relative to the job's items.
func (r *Run) Current(name string) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.j.Current = name
}

// Wait blocks while the job may not move: paused, waiting for its turn or
// held for a parity check. It is the pause point for work that does not
// move bytes.
func (r *Run) Wait(ctx context.Context) error { return r.m.gate(ctx, r.j) }

// Transfer is the pause point between two chunks of a copy. It waits for
// the turn, keeps to the speed limits of the job and of all jobs, and
// counts n bytes as done.
func (r *Run) Transfer(ctx context.Context, n int) error {
	for n > 0 {
		step := min(n, Chunk)
		if err := r.m.gate(ctx, r.j); err != nil {
			return err
		}
		if err := r.j.limiter.WaitN(ctx, step); err != nil {
			return ctxOr(ctx, err)
		}
		if err := r.m.global.WaitN(ctx, step); err != nil {
			return ctxOr(ctx, err)
		}
		r.m.mu.Lock()
		r.m.progress(r.j, int64(step), 0)
		r.m.mu.Unlock()
		n -= step
	}
	return nil
}

// ctxOr prefers the context error, because a limiter reports a cancelled
// wait with its own message.
func ctxOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Advance waits for the turn and counts work that is not held to a speed
// limit, such as files that are deleted or skipped.
func (r *Run) Advance(ctx context.Context, bytes, files int64) error {
	if err := r.m.gate(ctx, r.j); err != nil {
		return err
	}
	r.m.mu.Lock()
	r.m.progress(r.j, bytes, files)
	r.m.mu.Unlock()
	return nil
}

// Answer returns the decision for key, taken before the start or while
// the job ran. A decision for a folder covers everything inside it. It
// returns "" when the user still has to be asked.
func (r *Run) Answer(key string) string {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.j.answerFor(key)
}

func (j *job) answerFor(key string) string {
	for k := key; k != ""; {
		if c, ok := j.answers.Paths[k]; ok {
			return c
		}
		i := strings.LastIndex(k, "/")
		if i < 0 {
			break
		}
		k = k[:i]
	}
	return j.answers.All
}

// Ask puts the job on hold until the user answers the conflict, and
// gives its turn to the next job meanwhile.
func (r *Run) Ask(ctx context.Context, c Conflict) (string, error) {
	m, j := r.m, r.j
	m.mu.Lock()
	if a := j.answerFor(c.Key); a != "" {
		m.mu.Unlock()
		return a, nil
	}
	ch := make(chan string, 1)
	j.ask, j.Conflict = ch, &c
	j.State, j.allowed = Asking, false
	j.Rate, j.Left = 0, nil
	m.save(j)
	if m.hub != nil {
		payload := struct {
			ID       string   `json:"id"`
			Conflict Conflict `json:"conflict"`
		}{j.ID, c}
		if _, err := m.hub.PublishJSON(EventConflict, "", payload, j.Owner); err != nil {
			m.logf("jobs: %v", err)
		}
	}
	m.publish(EventState, j)
	m.schedule()
	m.mu.Unlock()
	select {
	case choice := <-ch:
		return choice, m.gate(ctx, j)
	case <-ctx.Done():
		m.mu.Lock()
		j.ask, j.Conflict = nil, nil
		m.mu.Unlock()
		return "", ctx.Err()
	}
}

// SetUndo links the job to the undo step that takes it back.
func (r *Run) SetUndo(id string) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.j.Undo = id
}

// SetResult stores what the job found out, for kinds such as a checksum
// run whose outcome is more than done or failed.
func (r *Run) SetResult(v any) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.j.Result = v
}

// TreeSize walks path without following links and returns the bytes of
// its files and how many entries there are below and at path.
func TreeSize(ctx context.Context, path string) (bytes, files int64, err error) {
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		files++
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			bytes += info.Size()
		}
		return nil
	})
	return bytes, files, err
}

// RemoveTree deletes path and everything below it one entry at a time, so
// the job can pause, be cancelled and count what is gone. A path that is
// gone already is no error.
func RemoveTree(ctx context.Context, run *Run, path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := RemoveTree(ctx, run, filepath.Join(path, e.Name())); err != nil {
				return err
			}
		}
	}
	if err := run.Wait(ctx); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	size := int64(0)
	if info.Mode().IsRegular() {
		size = info.Size()
	}
	return run.Advance(ctx, size, 1)
}
