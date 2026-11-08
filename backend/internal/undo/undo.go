package undo

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// Keep is how many steps a user can take back, as many as the menu of the
// prototype lists.
const Keep = 30

// JobKind takes back the steps that need more than a moment.
const JobKind = "undo"

func init() {
	migrate.Register(migrate.Main, migrate.SQL("undo/001_create",
		`CREATE TABLE undo_ops (
			id TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			count INTEGER NOT NULL,
			path TEXT NOT NULL,
			target TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX undo_ops_owner ON undo_ops (owner, created_at)`,
	))
	jobs.RegisterKind(JobKind, jobs.Kind{Run: runUndo, Elevate: elevateUndo})
}

// Step is one thing that can be taken back. Name, Count, Path and To are
// there to describe it: the first name, the number of entries, the folder
// it happened in and the new name or the folder things went to.
type Step struct {
	ID    string    `json:"id"`
	Kind  string    `json:"kind"`
	At    time.Time `json:"at"`
	Name  string    `json:"name"`
	Count int       `json:"count"`
	Path  string    `json:"path,omitempty"`
	To    string    `json:"to,omitempty"`
}

// Reverser takes back one kind of step. Inline runs inside the request
// and suits steps that take a moment, Job runs as a job for the others.
type Reverser struct {
	Inline func(ctx context.Context, d v1.Deps, u *users.User, data json.RawMessage) error
	Job    func(ctx context.Context, run *jobs.Run, data json.RawMessage) error
	// Elevate does Job with administrator rights, in the desktop app, once it
	// failed for lack of them.
	Elevate func(ctx context.Context, run *jobs.Run, data json.RawMessage) error
}

var (
	reversersMu sync.RWMutex
	reversers   = map[string]Reverser{}
)

// Register makes a kind of step undoable. It is meant for init functions.
func Register(kind string, r Reverser) {
	reversersMu.Lock()
	defer reversersMu.Unlock()
	if _, dup := reversers[kind]; dup || (r.Inline == nil) == (r.Job == nil) {
		panic(fmt.Sprintf("undo: bad or duplicate reverser %q", kind))
	}
	reversers[kind] = r
}

func reverserOf(kind string) (Reverser, bool) {
	reversersMu.RLock()
	defer reversersMu.RUnlock()
	r, ok := reversers[kind]
	return r, ok
}

// Log keeps the steps of every user.
type Log struct {
	deps v1.Deps
	db   *sql.DB
	hub  *hub.Hub
}

// For returns the log for the database in d, or nil without a database.
func For(d v1.Deps) *Log {
	if d.DB == nil {
		return nil
	}
	l, _ := v1.Shared(d, "undo", func() (*Log, error) { return &Log{deps: d, db: d.DB, hub: d.Hub}, nil })
	return l
}

// Record adds a step for owner and forgets the oldest beyond Keep. data
// is what the reverser needs and stays on the server.
func (l *Log) Record(owner string, s Step, data any) (Step, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Step{}, err
	}
	s.ID = strings.ToLower(rand.Text()[:16])
	s.At = time.Now()
	if _, err := l.db.Exec(`INSERT INTO undo_ops (id, owner, kind, name, count, path, target, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, owner, s.Kind, s.Name, s.Count, s.Path, s.To, string(raw), s.At.UnixMilli()); err != nil {
		return Step{}, err
	}
	if _, err := l.db.Exec(`DELETE FROM undo_ops WHERE owner = ? AND id NOT IN
		(SELECT id FROM undo_ops WHERE owner = ? ORDER BY created_at DESC LIMIT ?)`, owner, owner, Keep); err != nil {
		logger.Errorf("undo: %v", err)
	}
	l.changed(owner)
	return s, nil
}

// List returns the steps of owner, newest first.
func (l *Log) List(owner string) ([]Step, error) {
	rows, err := l.db.Query(`SELECT id, kind, name, count, path, target, created_at FROM undo_ops
		WHERE owner = ? ORDER BY created_at DESC, rowid DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	steps := []Step{}
	for rows.Next() {
		var s Step
		var at int64
		if err := rows.Scan(&s.ID, &s.Kind, &s.Name, &s.Count, &s.Path, &s.To, &at); err != nil {
			return nil, err
		}
		s.At = time.UnixMilli(at)
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

// ErrNothing means there is no step to take back.
var ErrNothing = v1.Coded(http.StatusNotFound, "nothingToUndo", "nothing to take back")

// Take takes back the step id of u, or the newest one when id is empty.
// A step that runs as a job is gone from the list once the job is queued,
// so it cannot be taken back twice.
func (l *Log) Take(ctx context.Context, u *users.User, id string) (Step, *jobs.Job, error) {
	s, data, err := l.find(u.Username, id)
	if err != nil {
		return Step{}, nil, err
	}
	rev, ok := reverserOf(s.Kind)
	if !ok {
		return Step{}, nil, v1.Coded(http.StatusConflict, "cannotUndo", "this step cannot be taken back")
	}
	if rev.Inline != nil {
		if err := rev.Inline(ctx, l.deps, u, data); err != nil {
			return Step{}, nil, err
		}
		l.forget(u.Username, s.ID)
		return s, nil, nil
	}
	m := jobs.For(l.deps)
	if m == nil {
		return Step{}, nil, errors.New("no job queue")
	}
	plan := &jobs.Plan{
		Params: jobParams{Kind: s.Kind, Data: data},
		Name:   s.Name,
		Items:  []jobs.Item{{Path: s.Path, Name: s.Name}},
	}
	j, err := m.Submit(u.Username, JobKind, plan, 0, false)
	if err != nil {
		return Step{}, nil, err
	}
	l.forget(u.Username, s.ID)
	return s, &j, nil
}

func (l *Log) find(owner, id string) (Step, json.RawMessage, error) {
	q := `SELECT id, kind, name, count, path, target, data, created_at FROM undo_ops WHERE owner = ?`
	args := []any{owner}
	if id != "" {
		q += ` AND id = ?`
		args = append(args, id)
	}
	q += ` ORDER BY created_at DESC, rowid DESC LIMIT 1`
	var s Step
	var data string
	var at int64
	err := l.db.QueryRow(q, args...).Scan(&s.ID, &s.Kind, &s.Name, &s.Count, &s.Path, &s.To, &data, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return Step{}, nil, ErrNothing
	}
	if err != nil {
		return Step{}, nil, err
	}
	s.At = time.UnixMilli(at)
	return s, json.RawMessage(data), nil
}

func (l *Log) forget(owner, id string) {
	if _, err := l.db.Exec(`DELETE FROM undo_ops WHERE owner = ? AND id = ?`, owner, id); err != nil {
		logger.Errorf("undo: %v", err)
	}
	l.changed(owner)
}

// Forget removes the steps of owner, for an account that is being deleted.
func (l *Log) Forget(owner string) error {
	_, err := l.db.Exec(`DELETE FROM undo_ops WHERE owner = ?`, owner)
	return err
}

// Rename moves the steps of a user to a new login name.
func (l *Log) Rename(from, to string) error {
	if _, err := l.db.Exec(`UPDATE undo_ops SET owner = ? WHERE owner = ?`, to, from); err != nil {
		return err
	}
	l.changed(to)
	return nil
}

type changedEvent struct {
	Count int `json:"count"`
}

func (l *Log) changed(owner string) {
	if l.hub == nil {
		return
	}
	var n int
	_ = l.db.QueryRow(`SELECT COUNT(*) FROM undo_ops WHERE owner = ?`, owner).Scan(&n)
	if _, err := l.hub.PublishJSON(EventChanged, "", changedEvent{Count: n}, owner); err != nil {
		logger.Errorf("undo: %v", err)
	}
}

type jobParams struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

func runUndo(ctx context.Context, run *jobs.Run) error { return undoWith(ctx, run, false) }

func elevateUndo(ctx context.Context, run *jobs.Run) error { return undoWith(ctx, run, true) }

var errNotElevatable = v1.Coded(http.StatusConflict, "notElevatable", "this cannot go on with administrator rights")

// undoWith takes the job's step back, with administrator rights when
// elevated.
func undoWith(ctx context.Context, run *jobs.Run, elevated bool) error {
	var p jobParams
	if err := run.Params(&p); err != nil {
		return err
	}
	rev, ok := reverserOf(p.Kind)
	if !ok || rev.Job == nil {
		return fmt.Errorf("cannot take back %s", p.Kind)
	}
	do := rev.Job
	if elevated {
		if do = rev.Elevate; do == nil {
			return errNotElevatable
		}
	}
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		err := do(ctx, run, p.Data)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}
