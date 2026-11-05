package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// Queue modes.
const (
	Serial   = "serial"
	Parallel = "parallel"
)

const (
	// Chunk is the step between two pause points of a transfer.
	Chunk = 1 << 20
	// maxParallel bounds the jobs that move at once side by side. Beyond
	// that the disks only seek more.
	maxParallel   = 4
	progressEvery = 250 * time.Millisecond
	saveEvery     = 2 * time.Second
	parityEvery   = 5 * time.Second
	keepFinished  = 7 * 24 * time.Hour
	// maxItemsKept is how many items a finished job may hold in memory. It
	// stays in the list for a week, and only a look at its items needs them.
	maxItemsKept = 1000
)

// Settings are the queue settings shared by all users.
type Settings struct {
	Mode string `json:"mode"`
	// Cap is the speed for all jobs together in bytes per second, 0 for
	// no limit.
	Cap int64 `json:"cap"`
	// ParityWait holds big copies and moves while a parity check runs.
	ParityWait bool `json:"parityWait"`
	// NotifyUnraid sends a notice to Unraid when a job that ran at least
	// NotifyAfter seconds ends.
	NotifyUnraid bool  `json:"notifyUnraid"`
	NotifyAfter  int64 `json:"notifyAfter"`
}

// Notice tells a finish hook about a job that ended.
type Notice struct {
	Job  Job
	Took time.Duration
}

// Options tune a manager for tests.
type Options struct {
	// ParityEvery is how often the parity hook is asked.
	ParityEvery time.Duration
}

type job struct {
	Job
	params  json.RawMessage
	answers answers
	items   []Item

	limiter   *rate.Limiter
	cancel    context.CancelFunc
	cancelled bool
	// forget drops the job from the list as soon as it has stopped.
	forget  bool
	live    bool
	allowed bool
	// pauses counts the pauses asked for, so a short one is not missed.
	pauses int
	ask    chan string

	// done counts the finished items, inflight what the current item
	// has moved so far.
	doneBytes, doneFiles         int64
	inflightBytes, inflightFiles int64

	publishedAt time.Time
	reportDue   bool
	savedAt     time.Time
	sampleAt    time.Time
	sampleBytes int64
}

// Manager runs the job queue. It is safe for concurrent use.
type Manager struct {
	deps v1.Deps
	db   *sql.DB
	hub  *hub.Hub
	opts Options

	ctx  context.Context
	stop context.CancelFunc
	wg   sync.WaitGroup

	mu         sync.Mutex
	jobs       map[string]*job
	settings   Settings
	global     *rate.Limiter
	changed    chan struct{}
	nextPos    int
	closed     bool
	parity     func() bool
	parityBusy bool
	finishHook []func(Notice)
}

// For returns the manager for the database in d, or nil when there is no
// database, as in tests that only look at the contract.
func For(d v1.Deps) *Manager {
	if d.DB == nil {
		return nil
	}
	m, err := v1.Shared(d, "jobs", func() (*Manager, error) { return New(d, Options{}) })
	if err != nil {
		logger.Errorf("jobs: %v", err)
		return nil
	}
	return m
}

// New loads the queue from d.DB and starts it. Jobs that were running when
// the server stopped go on where they were.
func New(d v1.Deps, opts Options) (*Manager, error) {
	if opts.ParityEvery <= 0 {
		opts.ParityEvery = parityEvery
	}
	m := &Manager{
		deps:    d,
		db:      d.DB,
		hub:     d.Hub,
		opts:    opts,
		jobs:    map[string]*job{},
		changed: make(chan struct{}),
	}
	m.ctx, m.stop = context.WithCancel(context.Background())
	m.settings = m.loadSettings()
	m.global = rate.NewLimiter(limitOf(m.settings.Cap), Chunk)

	m.exec(`DELETE FROM job_items WHERE job_id IN (SELECT id FROM jobs WHERE finished_at > 0 AND finished_at < ?)`,
		time.Now().Add(-keepFinished).UnixMilli())
	m.exec(`DELETE FROM jobs WHERE finished_at > 0 AND finished_at < ?`, time.Now().Add(-keepFinished).UnixMilli())
	list, err := m.load(m.ctx)
	if err != nil {
		return nil, fmt.Errorf("load jobs: %w", err)
	}
	m.mu.Lock()
	for _, j := range list {
		j.limiter = rate.NewLimiter(limitOf(j.Cap), Chunk)
		if j.State == Running || j.State == Asking {
			j.State = Queued
		}
		for i := range j.items {
			if j.items[i].State == ItemRunning {
				j.items[i].State = ItemPending
				j.items[i].Resumed = true
			}
		}
		j.recount()
		j.letGoOfItems()
		m.jobs[j.ID] = j
		m.nextPos = max(m.nextPos, j.Position+1)
	}
	m.schedule()
	m.mu.Unlock()

	m.wg.Add(1)
	go m.watchParity()
	return m, nil
}

// Close stops every running job at its next pause point and waits for
// them. Their state stays as it is, so the next start goes on with them.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.stop()
	m.wg.Wait()
}

// SetParityCheck installs the test for a running parity check or rebuild.
// The Unraid package provides it.
func (m *Manager) SetParityCheck(busy func() bool) {
	m.mu.Lock()
	m.parity = busy
	m.mu.Unlock()
	m.Reschedule()
}

// OnFinish adds a hook that hears about every job that ends, for messages
// after long jobs.
func (m *Manager) OnFinish(f func(Notice)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finishHook = append(m.finishHook, f)
}

// Reschedule asks the parity check again and lets waiting jobs go when it
// is over, for a caller that learns of the change before the next poll.
func (m *Manager) Reschedule() {
	m.mu.Lock()
	check := m.parity
	m.mu.Unlock()
	busy := check != nil && check()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parityBusy = busy
	m.schedule()
}

func (m *Manager) watchParity() {
	defer m.wg.Done()
	t := time.NewTicker(m.opts.ParityEvery)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			check := m.parity
			m.mu.Unlock()
			if check == nil {
				continue
			}
			busy := check()
			m.mu.Lock()
			if busy != m.parityBusy {
				m.parityBusy = busy
				m.schedule()
			}
			m.mu.Unlock()
		}
	}
}

// ParityBusy reports the last answer of the parity check.
func (m *Manager) ParityBusy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.parityBusy
}

// Submit adds a planned job to the end of the queue.
func (m *Manager) Submit(owner, kind string, p *Plan, capBytes int64, waitForParity bool) (Job, error) {
	if _, ok := kindOf(kind); !ok {
		return Job{}, v1.Errorf(http.StatusBadRequest, "unknown job kind")
	}
	if len(p.Items) == 0 {
		return Job{}, v1.Errorf(http.StatusBadRequest, "nothing to do")
	}
	if capBytes < 0 {
		return Job{}, v1.Errorf(http.StatusBadRequest, "cap must not be negative")
	}
	params, err := json.Marshal(p.Params)
	if err != nil {
		return Job{}, err
	}
	j := &job{params: params, limiter: rate.NewLimiter(limitOf(capBytes), Chunk)}
	j.ID = strings.ToLower(rand.Text()[:16])
	j.Kind, j.Owner, j.Dest = kind, owner, p.Dest
	j.State = Queued
	j.Cap, j.WaitForParity = capBytes, waitForParity
	j.CreatedAt = time.Now()
	j.answers = answers{Paths: p.Answers, All: p.Rule}
	if p.Rule == Ask {
		j.answers.All = ""
	}
	for i, it := range p.Items {
		it.Seq = i
		if it.State == "" {
			it.State = ItemPending
		}
		j.items = append(j.items, it)
	}
	j.Name = p.Name
	if j.Name == "" {
		j.Name = j.items[0].Name
		if j.Name == "" {
			j.Name = baseName(j.items[0].Path)
		}
	}
	j.recount()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Job{}, errors.New("job queue is shut down")
	}
	j.Position = m.nextPos
	m.nextPos++
	if err := m.insert(j); err != nil {
		return Job{}, err
	}
	m.jobs[j.ID] = j
	m.publish(EventCreated, j)
	m.schedule()
	return m.view(j, false), nil
}

// Get returns one job.
func (m *Manager) Get(id string, withItems bool) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return m.view(j, withItems), true
}

// List returns the jobs of owner, or of everybody when owner is empty:
// the open ones in queue order, then the finished ones, newest first.
func (m *Manager) List(owner string) []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	var open, done []*job
	for _, j := range m.jobs {
		if owner != "" && j.Owner != owner {
			continue
		}
		if j.State.Open() {
			open = append(open, j)
		} else {
			done = append(done, j)
		}
	}
	slices.SortFunc(open, func(a, b *job) int { return a.Position - b.Position })
	slices.SortFunc(done, func(a, b *job) int { return b.FinishedAt.Compare(a.FinishedAt) })
	out := make([]Job, 0, len(open)+len(done))
	for _, j := range append(open, done...) {
		out = append(out, m.view(j, false))
	}
	return out
}

// Pause holds a job at its next pause point.
func (m *Manager) Pause(id string) (Job, error) {
	return m.change(id, func(j *job) error {
		if j.State != Queued && j.State != Running {
			return v1.Errorf(http.StatusConflict, "only a waiting or running job can be paused")
		}
		j.State = Paused
		j.allowed = false
		j.pauses++
		j.Rate, j.Left = 0, nil
		return nil
	})
}

// Resume puts a paused job back into the queue.
func (m *Manager) Resume(id string) (Job, error) {
	return m.change(id, func(j *job) error {
		if j.State != Paused {
			return v1.Errorf(http.StatusConflict, "the job is not paused")
		}
		j.State = Queued
		return nil
	})
}

// Cancel stops a job. A running one stops at its next pause point.
func (m *Manager) Cancel(id string) (Job, error) {
	return m.change(id, func(j *job) error {
		if !j.State.Open() {
			return v1.Errorf(http.StatusConflict, "the job has ended already")
		}
		j.cancelled = true
		if j.live {
			j.cancel()
			return nil
		}
		m.end(j, nil)
		return nil
	})
}

// Retry runs the failed and unfinished items of an ended job again.
func (m *Manager) Retry(id string) (Job, error) {
	return m.change(id, func(j *job) error {
		if j.State != Failed && j.State != Cancelled {
			return v1.Errorf(http.StatusConflict, "only a failed or cancelled job can be retried")
		}
		for i := range j.items {
			it := &j.items[i]
			if it.State == ItemFailed || it.State == ItemRunning || it.State == ItemPending {
				it.Resumed = it.Resumed || it.State != ItemPending
				it.State, it.Error = ItemPending, ""
				m.saveItem(j, *it)
			}
		}
		j.State, j.Error, j.FinishedAt, j.cancelled = Queued, "", time.Time{}, false
		j.Position = m.nextPos
		m.nextPos++
		j.recount()
		return nil
	})
}

// Move puts an open job at index to among the open jobs.
func (m *Manager) Move(id string, to int) (Job, error) {
	return m.change(id, func(j *job) error {
		if !j.State.Open() {
			return v1.Errorf(http.StatusConflict, "the job has ended already")
		}
		open := m.openJobs()
		from := slices.Index(open, j)
		to = min(max(to, 0), len(open)-1)
		open = slices.Delete(open, from, from+1)
		open = slices.Insert(open, to, j)
		base := open[0].Position
		for _, o := range open {
			base = min(base, o.Position)
		}
		for i, o := range open {
			if o.Position != base+i {
				o.Position = base + i
				if o != j {
					m.save(o)
					m.publish(EventState, o)
				}
			}
		}
		return nil
	})
}

// SetCap limits one job to capBytes per second, 0 lifts the limit.
func (m *Manager) SetCap(id string, capBytes int64) (Job, error) {
	if capBytes < 0 {
		return Job{}, v1.Errorf(http.StatusBadRequest, "cap must not be negative")
	}
	return m.change(id, func(j *job) error {
		j.Cap = capBytes
		j.limiter.SetLimit(limitOf(capBytes))
		return nil
	})
}

// Decide answers the conflict a job waits on. With all, the choice also
// answers every later conflict of the job.
func (m *Manager) Decide(id, key, choice string, all bool) (Job, error) {
	if !ValidChoice(choice) {
		return Job{}, v1.Errorf(http.StatusBadRequest, "choice must be skip, replace, both or newer")
	}
	return m.change(id, func(j *job) error {
		if j.State != Asking || j.Conflict == nil || j.ask == nil {
			return v1.Errorf(http.StatusConflict, "the job is not waiting for an answer")
		}
		if key != "" && key != j.Conflict.Key {
			return v1.Errorf(http.StatusConflict, "the job waits for an answer about "+j.Conflict.Key)
		}
		if all {
			j.answers.All = choice
		} else {
			if j.answers.Paths == nil {
				j.answers.Paths = map[string]string{}
			}
			j.answers.Paths[j.Conflict.Key] = choice
		}
		j.ask <- choice
		j.ask, j.Conflict = nil, nil
		j.State = Queued
		return nil
	})
}

// Remove drops an ended job from the list.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return v1.Errorf(http.StatusNotFound, "no such job")
	}
	if j.State.Open() {
		return v1.Errorf(http.StatusConflict, "the job has not ended yet")
	}
	delete(m.jobs, id)
	m.delete(id)
	return nil
}

// ClearFinished drops the ended jobs of owner, or of everybody when owner
// is empty.
func (m *Manager) ClearFinished(owner string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, j := range m.jobs {
		if j.State.Open() || (owner != "" && j.Owner != owner) {
			continue
		}
		delete(m.jobs, id)
		m.delete(id)
		n++
	}
	return n
}

// Forget cancels every job of owner and drops them from the list, for an
// account that is being deleted. A running job stops at its next pause
// point and leaves the list then.
func (m *Manager) Forget(owner string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.jobs {
		if j.Owner != owner {
			continue
		}
		if j.live {
			j.cancelled, j.forget = true, true
			j.cancel()
			continue
		}
		delete(m.jobs, id)
		m.delete(id)
	}
	m.schedule()
}

// Rename moves the jobs of a user to a new login name. A job that is running
// goes on under it, so what it leaves behind lands with the same user.
func (m *Manager) Rename(from, to string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.db.Exec(`UPDATE jobs SET owner = ? WHERE owner = ?`, to, from); err != nil {
		return err
	}
	for _, j := range m.jobs {
		if j.Owner == from {
			j.Owner = to
		}
	}
	return nil
}

// Settings returns the queue settings.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// SetSettings changes the queue settings.
func (m *Manager) SetSettings(s Settings) (Settings, error) {
	if s.Mode != Serial && s.Mode != Parallel {
		return Settings{}, v1.Errorf(http.StatusBadRequest, "mode must be serial or parallel")
	}
	if s.Cap < 0 || s.NotifyAfter < 0 {
		return Settings{}, v1.Errorf(http.StatusBadRequest, "cap and notifyAfter must not be negative")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.saveSettings(s); err != nil {
		return Settings{}, err
	}
	m.settings = s
	m.global.SetLimit(limitOf(s.Cap))
	m.schedule()
	return s, nil
}

func (m *Manager) change(id string, f func(*job) error) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return Job{}, v1.Errorf(http.StatusNotFound, "no such job")
	}
	if err := f(j); err != nil {
		return Job{}, err
	}
	m.save(j)
	m.publish(EventState, j)
	m.schedule()
	return m.view(j, false), nil
}

// openJobs returns the open jobs in queue order.
func (m *Manager) openJobs() []*job {
	var out []*job
	for _, j := range m.jobs {
		if j.State.Open() {
			out = append(out, j)
		}
	}
	slices.SortFunc(out, func(a, b *job) int { return a.Position - b.Position })
	return out
}

// schedule decides which jobs may move. Serial mode lets the first job in
// the queue move, parallel mode up to maxParallel, and a running parity
// check holds the big copies and moves. A job that loses its turn stops at its
// next pause point. Call with m.mu held.
func (m *Manager) schedule() {
	slots := 1
	if m.settings.Mode == Parallel {
		slots = maxParallel
	}
	busy := m.parityBusy && m.settings.ParityWait
	place := 0
	for _, j := range m.openJobs() {
		if j.State != Queued && j.State != Running {
			continue
		}
		k, known := kindOf(j.Kind)
		held := busy && k.Heavy && (j.WaitForParity || j.TotalBytes >= BigJob)
		allow := known && !held && slots > 0 && !m.closed
		state, reason, was := j.State, j.Reason, j.Place
		j.Reason = ""
		if held {
			j.Reason = "parity"
		}
		if allow {
			slots--
			j.State, j.Place = Running, 0
		} else {
			place++
			j.State, j.Place = Queued, place
			j.Rate, j.Left = 0, nil
		}
		j.allowed = allow
		if allow && !j.live {
			m.start(j)
		}
		if state != j.State || reason != j.Reason || was != j.Place {
			m.save(j)
			m.publish(EventState, j)
		}
	}
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *Manager) start(j *job) {
	k, _ := kindOf(j.Kind)
	ctx, cancel := context.WithCancel(m.ctx)
	j.cancel, j.live = cancel, true
	if j.StartedAt.IsZero() {
		j.StartedAt = time.Now()
	}
	j.inflightBytes, j.inflightFiles = 0, 0
	j.recount()
	j.sampleAt, j.sampleBytes = time.Now(), j.DoneBytes
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		err := runKind(ctx, k, &Run{m: m, j: j})
		m.mu.Lock()
		hooks := m.end(j, err)
		m.mu.Unlock()
		for _, f := range hooks {
			f()
		}
	}()
}

func runKind(ctx context.Context, k Kind, r *Run) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("job crashed: %v", p)
			logger.Errorf("jobs: %s %s: %v", r.j.Kind, r.j.ID, p)
		}
	}()
	return k.Run(ctx, r)
}

// end closes a job after its run returned, or right away when it never
// ran. It returns the finish hooks to call once m.mu is released.
func (m *Manager) end(j *job, err error) []func() {
	j.live, j.allowed = false, false
	if j.cancel != nil {
		j.cancel()
	}
	if m.closed && !j.cancelled {
		return nil
	}
	j.inflightBytes, j.inflightFiles = 0, 0
	j.recount()
	switch {
	case j.cancelled:
		j.State = Cancelled
	case err != nil:
		j.State, j.Error = Failed, errorText(err)
	case j.FailedItems > 0:
		j.State = Failed
		if j.Error == "" {
			j.Error = fmt.Sprintf("%d of %d items failed", j.FailedItems, len(j.items))
		}
	default:
		j.State = Done
	}
	j.FinishedAt = time.Now()
	j.Rate, j.Left, j.Current, j.Place, j.Reason = 0, nil, "", 0, ""
	j.ask, j.Conflict = nil, nil
	m.save(j)
	j.letGoOfItems()
	m.publish(EventDone, j)
	if j.forget {
		delete(m.jobs, j.ID)
		m.delete(j.ID)
	}
	m.schedule()
	n := Notice{Job: m.view(j, false), Took: j.FinishedAt.Sub(j.StartedAt)}
	if j.StartedAt.IsZero() {
		n.Took = 0
	}
	var calls []func()
	for _, f := range m.finishHook {
		calls = append(calls, func() { f(n) })
	}
	return calls
}

func errorText(err error) string {
	var p *v1.Problem
	if errors.As(err, &p) {
		return p.Error()
	}
	return err.Error()
}

// gate blocks while the job may not move.
func (m *Manager) gate(ctx context.Context, j *job) error {
	var seen int
	_, err := m.held(ctx, j, &seen)
	return err
}

// held waits for the turn of j and reports whether it had to, or was paused
// since the pauses it has seen, however short that lasted.
func (m *Manager) held(ctx context.Context, j *job, seen *int) (bool, error) {
	waited := false
	for {
		m.mu.Lock()
		ok, ch := j.allowed, m.changed
		if ok && j.pauses != *seen {
			*seen, waited = j.pauses, true
		}
		m.mu.Unlock()
		if ok {
			return waited, ctx.Err()
		}
		waited = true
		select {
		case <-ch:
		case <-ctx.Done():
			return waited, ctx.Err()
		}
	}
}

// progress counts work and reports it at most four times a second. Call
// with m.mu held.
func (m *Manager) progress(j *job, bytes, files int64) {
	j.inflightBytes += bytes
	j.inflightFiles += files
	j.DoneBytes = j.doneBytes + j.inflightBytes
	j.DoneFiles = j.doneFiles + j.inflightFiles
	now := time.Now()
	if dt := now.Sub(j.sampleAt); dt >= progressEvery {
		sample := float64(j.DoneBytes-j.sampleBytes) / dt.Seconds()
		if j.Rate == 0 {
			j.Rate = sample
		} else {
			j.Rate = 0.6*j.Rate + 0.4*sample
		}
		j.sampleAt, j.sampleBytes = now, j.DoneBytes
		j.Left = nil
		if j.Rate > 0 && j.TotalBytes > 0 {
			left := float64(max(j.TotalBytes-j.DoneBytes, 0)) / j.Rate
			j.Left = &left
		}
	}
	if now.Sub(j.publishedAt) >= progressEvery {
		m.publishProgress(j)
	}
	if now.Sub(j.savedAt) >= saveEvery {
		m.save(j)
	}
}

// letGoOfItems drops the items of a big job that is through. Only a failed or
// cancelled job can run them again.
func (j *job) letGoOfItems() {
	if j.State == Done && len(j.items) > maxItemsKept {
		j.items = nil
	}
}

// recount sums the items into the totals.
func (j *job) recount() {
	j.TotalBytes, j.TotalFiles, j.doneBytes, j.doneFiles, j.FailedItems = 0, 0, 0, 0, 0
	for i := range j.items {
		j.tally(&j.items[i], 1)
	}
	j.addUp()
	j.Count = len(j.items)
}

// tally adds what an item counts for to the totals, or takes it out again
// with sign -1.
func (j *job) tally(it *Item, sign int64) {
	if it.State == ItemFailed {
		j.FailedItems += int(sign)
	}
	j.TotalBytes += sign * it.Bytes
	j.TotalFiles += sign * it.Files
	if it.State == ItemDone || it.State == ItemSkipped {
		j.doneBytes += sign * it.Bytes
		j.doneFiles += sign * it.Files
	}
}

func (j *job) addUp() {
	j.DoneBytes = j.doneBytes + j.inflightBytes
	j.DoneFiles = j.doneFiles + j.inflightFiles
}

func (m *Manager) view(j *job, withItems bool) Job {
	v := j.Job
	if j.Left != nil {
		left := *j.Left
		v.Left = &left
	}
	if j.Conflict != nil {
		c := *j.Conflict
		v.Conflict = &c
	}
	v.Items = nil
	if withItems {
		v.Items = slices.Clone(j.items)
		if j.items == nil {
			v.Items = m.storedItems(j.ID)
		}
	} else if k, _ := kindOf(j.Kind); k.BigResult {
		v.Result = nil
	}
	return v
}

func (m *Manager) publish(typ string, j *job) {
	if m.hub == nil {
		return
	}
	if _, err := m.hub.PublishJSON(typ, "", m.view(j, false), j.Owner); err != nil {
		m.logf("jobs: %v", err)
	}
	j.publishedAt = time.Now()
}

// Progress is the payload of job.progress.
type Progress struct {
	ID         string   `json:"id"`
	TotalBytes int64    `json:"totalBytes"`
	DoneBytes  int64    `json:"doneBytes"`
	TotalFiles int64    `json:"totalFiles"`
	DoneFiles  int64    `json:"doneFiles"`
	Rate       float64  `json:"rate"`
	Left       *float64 `json:"left"`
	Current    string   `json:"current,omitempty"`
}

func (m *Manager) publishProgress(j *job) {
	j.publishedAt = time.Now()
	if m.hub == nil {
		return
	}
	p := Progress{ID: j.ID, TotalBytes: j.TotalBytes, DoneBytes: j.DoneBytes, TotalFiles: j.TotalFiles,
		DoneFiles: j.DoneFiles, Rate: j.Rate, Left: j.Left, Current: j.Current}
	if _, err := m.hub.PublishJSON(EventProgress, "", p, j.Owner); err != nil {
		m.logf("jobs: %v", err)
	}
}

// reportSoon publishes the progress of j when the next report is due, since
// nothing else may report it for a while. Call with m.mu held.
func (m *Manager) reportSoon(j *job) {
	if j.reportDue {
		return
	}
	j.reportDue = true
	time.AfterFunc(progressEvery, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		j.reportDue = false
		switch {
		case !j.live:
		case time.Since(j.publishedAt) >= progressEvery:
			m.publishProgress(j)
		default:
			m.reportSoon(j)
		}
	})
}

func (m *Manager) logf(format string, args ...any) {
	logger.Errorf(format, args...)
}

func limitOf(capBytes int64) rate.Limit {
	if capBytes <= 0 {
		return rate.Inf
	}
	return rate.Limit(capBytes)
}

func baseName(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
