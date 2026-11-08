package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// State is where a job stands.
type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Paused    State = "paused"
	Asking    State = "asking"
	Done      State = "done"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

// Open reports whether the job still has work ahead.
func (s State) Open() bool { return s == Queued || s == Running || s == Paused || s == Asking }

// Item states. A job works through its items in order and records each
// one, so a restart or a retry knows where to go on.
const (
	ItemPending = "pending"
	ItemRunning = "running"
	ItemDone    = "done"
	ItemSkipped = "skipped"
	ItemFailed  = "failed"
)

// Answers to a conflict. Ask is only valid as a rule chosen before the job
// starts.
const (
	Ask     = "ask"
	Skip    = "skip"
	Replace = "replace"
	Both    = "both"
	Newer   = "newer"
)

// ValidChoice reports whether c answers a conflict.
func ValidChoice(c string) bool { return c == Skip || c == Replace || c == Both || c == Newer }

// ValidRule reports whether r can be chosen for all conflicts up front.
func ValidRule(r string) bool { return r == Ask || ValidChoice(r) }

// BigJob is the size from which a copy or move waits for a parity check.
const BigJob = 2 << 30

// Item is one entry a job works on, usually one selected file or folder.
type Item struct {
	Seq   int    `json:"seq"`
	Path  string `json:"path"`
	Name  string `json:"name,omitempty"`
	State string `json:"state"`
	Bytes int64  `json:"bytes"`
	Files int64  `json:"files"`
	Error string `json:"error,omitempty"`
	// ErrorCode and ErrorParams say Error the way a problem does, for an
	// interface that shows it in its own language.
	ErrorCode   string         `json:"errorCode,omitempty"`
	ErrorParams map[string]any `json:"errorParams,omitempty"`
	// Ref is kind specific, for example the trash entry to restore.
	Ref string `json:"-"`
	// Resumed marks an item that was cut off before, so the kind can tell
	// its own half finished work from what was there before.
	Resumed bool `json:"-"`
}

// Side describes one of the two entries in a conflict.
type Side struct {
	Path    string    `json:"path"`
	Dir     bool      `json:"dir"`
	Size    int64     `json:"size"`
	MTime   time.Time `json:"mtime"`
	Entries int       `json:"entries,omitempty"`
}

// Conflict is a name that exists at the target already.
type Conflict struct {
	// Key is the path relative to the destination. Answers are given for
	// it, and an answer for a folder covers what lies inside.
	Key    string `json:"key"`
	Source Side   `json:"source"`
	Target Side   `json:"target"`
	// Newer names the side with the later modification time, if any.
	Newer string `json:"newer,omitempty"`
}

// NewConflict fills in which side is newer. Times within a second count
// as equal, because some file systems only keep whole or even seconds.
func NewConflict(key string, source, target Side) Conflict {
	c := Conflict{Key: key, Source: source, Target: target}
	switch d := source.MTime.Sub(target.MTime); {
	case d > time.Second:
		c.Newer = "source"
	case d < -time.Second:
		c.Newer = "target"
	}
	return c
}

// Job is the public view of a job.
type Job struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Owner string `json:"owner"`
	State State  `json:"state"`
	// Reason says why a queued job does not run: "parity" while it waits
	// for a parity check, empty while it waits for its turn.
	Reason string `json:"reason,omitempty"`
	// Place is the place in the waiting line, counted from 1.
	Place int `json:"place,omitempty"`
	// Position orders the open jobs.
	Position      int            `json:"position"`
	Name          string         `json:"name"`
	Count         int            `json:"count"`
	Dest          string         `json:"dest,omitempty"`
	TotalBytes    int64          `json:"totalBytes"`
	DoneBytes     int64          `json:"doneBytes"`
	TotalFiles    int64          `json:"totalFiles"`
	DoneFiles     int64          `json:"doneFiles"`
	Rate          float64        `json:"rate"`
	Left          *float64       `json:"left"`
	Current       string         `json:"current,omitempty"`
	Cap           int64          `json:"cap"`
	WaitForParity bool           `json:"waitForParity"`
	FailedItems   int            `json:"failedItems"`
	Error         string         `json:"error,omitempty"`
	ErrorCode     string         `json:"errorCode,omitempty"`
	ErrorParams   map[string]any `json:"errorParams,omitempty"`
	Conflict      *Conflict      `json:"conflict,omitempty"`
	Undo          string         `json:"undo,omitempty"`
	Result        any            `json:"result,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	StartedAt     time.Time      `json:"startedAt,omitzero"`
	FinishedAt    time.Time      `json:"finishedAt,omitzero"`
	Items         []Item         `json:"items,omitempty"`
}

// Plan is what a kind hands the manager for a new job.
type Plan struct {
	// Params are stored with the job and read back by Run.
	Params any
	// Name and Dest describe the job in lists. Name defaults to the name
	// of the first item.
	Name  string
	Dest  string
	Items []Item
	// Answers are decisions taken before the start, keyed like
	// Conflict.Key.
	Answers map[string]string
	// Rule answers every conflict without an answer of its own.
	Rule string
}

// Kind is one sort of job.
type Kind struct {
	// Prepare checks a request from POST /jobs and plans the job. It runs
	// inside the request, so it must not walk big trees. Kinds without it
	// are only started by other packages.
	Prepare func(ctx context.Context, d v1.Deps, u *users.User, params json.RawMessage) (*Plan, error)
	// Run does the work. It is called again with the same items after a
	// restart or a retry and has to skip what is done already.
	Run func(ctx context.Context, run *Run) error
	// Elevate does the open items again with administrator rights, in the
	// desktop app, once the job failed for lack of them. Kinds without it
	// cannot go on that way.
	Elevate func(ctx context.Context, run *Run) error
	// Heavy kinds wait for a parity check once they are big.
	Heavy bool
	// BigResult kinds can find megabytes, which would go out with every
	// event and every list of jobs. Clients read it from the job itself.
	BigResult bool
}

var (
	kindsMu sync.RWMutex
	kinds   = map[string]Kind{}
)

// RegisterKind makes a kind known. It is meant for init functions and
// panics on a duplicate, because that is a programming error.
func RegisterKind(name string, k Kind) {
	kindsMu.Lock()
	defer kindsMu.Unlock()
	if _, dup := kinds[name]; dup || k.Run == nil {
		panic(fmt.Sprintf("jobs: bad or duplicate kind %q", name))
	}
	kinds[name] = k
}

func kindOf(name string) (Kind, bool) {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	k, ok := kinds[name]
	return k, ok
}

// Decode reads job params strictly, so a misspelt field is an error.
func Decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := strictUnmarshal(raw, v); err != nil {
		return v1.Errorf(http.StatusBadRequest, "invalid params: "+err.Error())
	}
	return nil
}
