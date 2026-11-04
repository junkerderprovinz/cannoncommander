package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

// script is the behaviour of a test job, picked by the name in its params.
type script func(ctx context.Context, run *Run) error

var (
	scriptsMu sync.Mutex
	scripts   = map[string]script{}
)

type testParams struct {
	Script string `json:"script"`
}

func init() {
	RegisterKind("test", Kind{Run: func(ctx context.Context, run *Run) error {
		var p testParams
		if err := run.Params(&p); err != nil {
			return err
		}
		scriptsMu.Lock()
		f := scripts[p.Script]
		scriptsMu.Unlock()
		return f(ctx, run)
	}})
	RegisterKind("bigresult", Kind{BigResult: true, Run: func(ctx context.Context, run *Run) error {
		run.SetResult(map[string]int{"hits": 1})
		run.Finish(0, ItemDone, nil)
		return nil
	}})
	RegisterKind("heavy", Kind{Heavy: true, Run: func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			if err := run.Transfer(ctx, 1); err != nil {
				return err
			}
			run.Finish(it.Seq, ItemDone, nil)
		}
		return nil
	}})
}

func addScript(t *testing.T, f script) string {
	t.Helper()
	scriptsMu.Lock()
	defer scriptsMu.Unlock()
	name := fmt.Sprintf("%s#%d", t.Name(), len(scripts))
	scripts[name] = f
	return name
}

type env struct {
	deps v1.Deps
	m    *Manager
	hub  *hub.Hub
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d := v1.Deps{DB: testutil.MainDB(t), Hub: hub.New(hub.Options{})}
	e := &env{deps: d, hub: d.Hub}
	e.restart(t)
	return e
}

// restart closes the manager, if there is one, and opens a new one on the
// same database, as a server restart would.
func (e *env) restart(t *testing.T) {
	t.Helper()
	if e.m != nil {
		e.m.Close()
	}
	m, err := New(e.deps, Options{ParityEvery: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	e.m = m
	t.Cleanup(m.Close)
}

func (e *env) submit(t *testing.T, kind, script string, items int) Job {
	t.Helper()
	p := &Plan{Params: testParams{Script: script}}
	for i := range items {
		p.Items = append(p.Items, Item{Path: "/mnt/user/x/" + string(rune('a'+i))})
	}
	j, err := e.m.Submit("anna", kind, p, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (e *env) waitFor(t *testing.T, id string, ok func(Job) bool) Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, found := e.m.Get(id, true)
		if found && ok(j) {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s stuck: %+v", id, j)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (e *env) waitState(t *testing.T, id string, s State) Job {
	t.Helper()
	return e.waitFor(t, id, func(j Job) bool { return j.State == s })
}

// gated runs every item as one transfer of a byte and waits on release
// before each item.
func gated(release <-chan struct{}) script {
	return func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			if it.State != ItemPending {
				continue
			}
			run.Begin(it.Seq)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			if err := run.Transfer(ctx, 1); err != nil {
				return err
			}
			run.Finish(it.Seq, ItemDone, nil)
		}
		return nil
	}
}

func TestSerialModeRunsOneJobAtATime(t *testing.T) {
	e := newEnv(t)
	release := make(chan struct{})
	name := addScript(t, gated(release))
	a := e.submit(t, "test", name, 1)
	b := e.submit(t, "test", name, 1)
	e.waitState(t, a.ID, Running)
	jb := e.waitState(t, b.ID, Queued)
	if jb.Place != 1 {
		t.Fatalf("second job waits at place %d", jb.Place)
	}
	release <- struct{}{}
	e.waitState(t, a.ID, Done)
	e.waitState(t, b.ID, Running)
	release <- struct{}{}
	e.waitState(t, b.ID, Done)
}

func TestParallelModeRunsJobsSideBySide(t *testing.T) {
	e := newEnv(t)
	if _, err := e.m.SetSettings(Settings{Mode: Parallel, ParityWait: true}); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	name := addScript(t, gated(release))
	a := e.submit(t, "test", name, 1)
	b := e.submit(t, "test", name, 1)
	e.waitState(t, a.ID, Running)
	e.waitState(t, b.ID, Running)
	close(release)
	e.waitState(t, a.ID, Done)
	e.waitState(t, b.ID, Done)
}

// streaming moves bytes until stop is closed, so tests can watch it pause.
func streaming(stop <-chan struct{}) script {
	return func(ctx context.Context, run *Run) error {
		it := run.Items()[0]
		run.Begin(it.Seq)
		for {
			select {
			case <-stop:
				run.Finish(it.Seq, ItemDone, nil)
				return nil
			default:
			}
			if err := run.Transfer(ctx, 1024); err != nil {
				return err
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func TestPauseHoldsAJobUntilItIsResumed(t *testing.T) {
	e := newEnv(t)
	stop := make(chan struct{})
	j := e.submit(t, "test", addScript(t, streaming(stop)), 1)
	e.waitFor(t, j.ID, func(j Job) bool { return j.DoneBytes > 0 })
	if _, err := e.m.Pause(j.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	before, _ := e.m.Get(j.ID, false)
	time.Sleep(50 * time.Millisecond)
	after, _ := e.m.Get(j.ID, false)
	if after.State != Paused || after.DoneBytes != before.DoneBytes {
		t.Fatalf("paused job moved on: %d then %d bytes, %s", before.DoneBytes, after.DoneBytes, after.State)
	}
	if _, err := e.m.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	e.waitFor(t, j.ID, func(j Job) bool { return j.DoneBytes > after.DoneBytes })
	close(stop)
	e.waitState(t, j.ID, Done)
}

func TestCancelStopsARunningJob(t *testing.T) {
	e := newEnv(t)
	sawCancel := make(chan bool, 1)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		<-ctx.Done()
		sawCancel <- run.Cancelled()
		return ctx.Err()
	})
	j := e.submit(t, "test", name, 1)
	e.waitState(t, j.ID, Running)
	if _, err := e.m.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	e.waitState(t, j.ID, Cancelled)
	if !<-sawCancel {
		t.Fatal("the kind could not tell a cancel from a shutdown")
	}
	if _, err := e.m.Cancel(j.ID); err == nil {
		t.Fatal("an ended job was cancelled twice")
	}
}

func TestAJobMovedToTheFrontTakesTheTurn(t *testing.T) {
	e := newEnv(t)
	stop := make(chan struct{})
	first := e.submit(t, "test", addScript(t, streaming(stop)), 1)
	e.waitState(t, first.ID, Running)
	release := make(chan struct{})
	second := e.submit(t, "test", addScript(t, gated(release)), 1)
	e.waitState(t, second.ID, Queued)

	if _, err := e.m.Move(second.ID, 0); err != nil {
		t.Fatal(err)
	}
	e.waitState(t, second.ID, Running)
	held := e.waitState(t, first.ID, Queued)
	time.Sleep(30 * time.Millisecond)
	if now, _ := e.m.Get(first.ID, false); now.DoneBytes > held.DoneBytes+Chunk {
		t.Fatalf("the job that lost its turn kept moving: %d then %d", held.DoneBytes, now.DoneBytes)
	}
	close(release)
	e.waitState(t, second.ID, Done)
	e.waitState(t, first.ID, Running)
	close(stop)
	e.waitState(t, first.ID, Done)
}

func TestSpeedLimitOfAJob(t *testing.T) {
	e := newEnv(t)
	const size = 3 * Chunk
	name := addScript(t, func(ctx context.Context, run *Run) error {
		it := run.Items()[0]
		run.Measure(it.Seq, size, 1)
		run.Begin(it.Seq)
		for range size / (64 << 10) {
			if err := run.Transfer(ctx, 64<<10); err != nil {
				return err
			}
		}
		run.Finish(it.Seq, ItemDone, nil)
		return nil
	})
	p := &Plan{Params: testParams{Script: name}, Items: []Item{{Path: "/mnt/user/x/a"}}}
	start := time.Now()
	j, err := e.m.Submit("anna", "test", p, 4*Chunk, false)
	if err != nil {
		t.Fatal(err)
	}
	e.waitState(t, j.ID, Done)
	// The first chunk comes from the burst, the other two take half a second.
	if took := time.Since(start); took < 400*time.Millisecond {
		t.Fatalf("3 MiB at 4 MiB/s took only %s", took)
	}
}

func TestSettingsAreValidatedAndKept(t *testing.T) {
	e := newEnv(t)
	if _, err := e.m.SetSettings(Settings{Mode: "fast"}); err == nil {
		t.Fatal("an unknown mode was accepted")
	}
	want := Settings{Mode: Parallel, Cap: 50 << 20, ParityWait: false}
	if _, err := e.m.SetSettings(want); err != nil {
		t.Fatal(err)
	}
	e.restart(t)
	if got := e.m.Settings(); got != want {
		t.Fatalf("settings after restart: %+v", got)
	}
}

func TestBigJobsWaitForTheParityCheck(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	busy := true
	e.m.SetParityCheck(func() bool { mu.Lock(); defer mu.Unlock(); return busy })

	p := &Plan{Params: struct{}{}, Items: []Item{{Path: "/mnt/user/x/a", Bytes: BigJob}}}
	big, err := e.m.Submit("anna", "heavy", p, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	small, err := e.m.Submit("anna", "heavy", &Plan{Params: struct{}{}, Items: []Item{{Path: "/mnt/user/x/b"}}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	asked, err := e.m.Submit("anna", "heavy", &Plan{Params: struct{}{}, Items: []Item{{Path: "/mnt/user/x/c"}}}, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	e.waitState(t, small.ID, Done)
	for _, id := range []string{big.ID, asked.ID} {
		if j := e.waitState(t, id, Queued); j.Reason != "parity" {
			t.Fatalf("held job gives reason %q", j.Reason)
		}
	}
	mu.Lock()
	busy = false
	mu.Unlock()
	e.waitState(t, big.ID, Done)
	e.waitState(t, asked.ID, Done)
}

func TestAConflictWaitsForTheAnswer(t *testing.T) {
	e := newEnv(t)
	client, _, _ := e.hub.Connect("tab", "anna", 0)
	answers := make(chan string, 2)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		for _, key := range []string{"Urlaub/a.jpg", "Urlaub/b.jpg"} {
			c, err := run.Ask(ctx, NewConflict(key, Side{Path: "/src/" + key}, Side{Path: "/dst/" + key}))
			if err != nil {
				return err
			}
			answers <- c
		}
		return nil
	})
	j := e.submit(t, "test", name, 1)
	asking := e.waitState(t, j.ID, Asking)
	if asking.Conflict == nil || asking.Conflict.Key != "Urlaub/a.jpg" {
		t.Fatalf("job waits on %+v", asking.Conflict)
	}
	var sawConflict bool
	for !sawConflict {
		select {
		case ev := <-client.Events:
			sawConflict = ev.Type == EventConflict
		case <-time.After(5 * time.Second):
			t.Fatal("no job.conflict event")
		}
	}
	if _, err := e.m.Decide(j.ID, "Urlaub/other", Skip, false); err == nil {
		t.Fatal("an answer to another conflict was taken")
	}
	if _, err := e.m.Decide(j.ID, "", Replace, true); err != nil {
		t.Fatal(err)
	}
	if a, b := <-answers, <-answers; a != Replace || b != Replace {
		t.Fatalf("answers %s, %s", a, b)
	}
	e.waitState(t, j.ID, Done)
}

func TestAnAnswerForAFolderCoversItsContent(t *testing.T) {
	j := &job{answers: answers{Paths: map[string]string{"Urlaub": Replace, "Urlaub/2025/x.jpg": Skip}, All: Both}}
	for key, want := range map[string]string{
		"Urlaub":            Replace,
		"Urlaub/a.jpg":      Replace,
		"Urlaub/2025/x.jpg": Skip,
		"Urlaub/2025/y.jpg": Replace,
		"Urlaubsfotos":      Both,
	} {
		if got := j.answerFor(key); got != want {
			t.Errorf("%s: %s, want %s", key, got, want)
		}
	}
}

func TestAJobGoesOnAfterARestart(t *testing.T) {
	e := newEnv(t)
	type seen struct {
		path    string
		resumed bool
	}
	runs := make(chan []seen, 2)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		var got []seen
		for _, it := range run.Items() {
			if it.State != ItemPending {
				continue
			}
			got = append(got, seen{it.Path, it.Resumed})
			run.Begin(it.Seq)
			if it.Seq == 1 && !it.Resumed {
				runs <- got
				<-ctx.Done()
				return ctx.Err()
			}
			run.Finish(it.Seq, ItemDone, nil)
		}
		runs <- got
		return nil
	})
	j := e.submit(t, "test", name, 3)
	first := <-runs
	if len(first) != 2 {
		t.Fatalf("first run saw %+v", first)
	}
	e.restart(t)
	second := <-runs
	if len(second) != 2 || !second[0].resumed || second[0].path != "/mnt/user/x/b" || second[1].resumed {
		t.Fatalf("after the restart the job saw %+v", second)
	}
	if done := e.waitState(t, j.ID, Done); done.StartedAt.IsZero() {
		t.Fatal("start time lost")
	}
}

func TestRetryRunsTheFailedItemsAgain(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	fail := true
	name := addScript(t, func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			if it.State != ItemPending {
				continue
			}
			mu.Lock()
			var err error
			if fail && it.Seq == 1 {
				err = errors.New("disk is full")
			}
			mu.Unlock()
			run.Finish(it.Seq, ItemDone, err)
		}
		return nil
	})
	j := e.submit(t, "test", name, 2)
	failed := e.waitState(t, j.ID, Failed)
	if failed.FailedItems != 1 || failed.Items[1].Error != "disk is full" || failed.Items[0].State != ItemDone {
		t.Fatalf("failed job: %+v", failed)
	}
	mu.Lock()
	fail = false
	mu.Unlock()
	if _, err := e.m.Retry(j.ID); err != nil {
		t.Fatal(err)
	}
	if done := e.waitState(t, j.ID, Done); done.Error != "" || done.FailedItems != 0 {
		t.Fatalf("retried job: %+v", done)
	}
}

func TestProgressIsReportedAtMostFourTimesASecond(t *testing.T) {
	e := newEnv(t)
	client, _, _ := e.hub.Connect("tab", "anna", 0)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		it := run.Items()[0]
		run.Measure(it.Seq, 1<<20, 1)
		run.Begin(it.Seq)
		deadline := time.Now().Add(600 * time.Millisecond)
		for time.Now().Before(deadline) {
			if err := run.Transfer(ctx, 16); err != nil {
				return err
			}
		}
		return nil
	})
	j := e.submit(t, "test", name, 1)
	e.waitState(t, j.ID, Done)
	n := 0
	for {
		select {
		case ev := <-client.Events:
			if ev.Type == EventProgress {
				n++
			}
			continue
		default:
		}
		break
	}
	// 600 ms allow three reports, one more for the end of the item.
	if n == 0 || n > 5 {
		t.Fatalf("%d progress events in 600 ms", n)
	}
}

func TestEndedJobsCanBeRemoved(t *testing.T) {
	e := newEnv(t)
	release := make(chan struct{})
	name := addScript(t, gated(release))
	a := e.submit(t, "test", name, 1)
	if err := e.m.Remove(a.ID); err == nil {
		t.Fatal("an open job was removed")
	}
	close(release)
	e.waitState(t, a.ID, Done)
	b := e.submit(t, "test", name, 1)
	e.waitState(t, b.ID, Done)
	if n := e.m.ClearFinished("bert"); n != 0 {
		t.Fatalf("cleared %d jobs of another user", n)
	}
	if n := e.m.ClearFinished("anna"); n != 2 {
		t.Fatalf("cleared %d jobs", n)
	}
	e.restart(t)
	if list := e.m.List(""); len(list) != 0 {
		t.Fatalf("removed jobs came back: %+v", list)
	}
}

func TestForgetDropsEveryJobOfTheOwner(t *testing.T) {
	e := newEnv(t)
	release := make(chan struct{})
	name := addScript(t, gated(release))
	running := e.submit(t, "test", name, 1)
	waiting := e.submit(t, "test", name, 1)
	e.waitState(t, running.ID, Running)
	other, err := e.m.Submit("bert", "test", &Plan{Params: testParams{Script: name}, Items: []Item{{Path: "/mnt/user/b"}}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	e.m.Forget("anna")
	deadline := time.Now().Add(10 * time.Second)
	for len(e.m.List("anna")) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("jobs of anna are left: %+v", e.m.List("anna"))
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := e.m.Get(waiting.ID, false); ok {
		t.Fatal("the queued job is still there")
	}
	e.waitState(t, other.ID, Running)
	close(release)
	e.waitState(t, other.ID, Done)
	e.restart(t)
	if list := e.m.List("anna"); len(list) != 0 {
		t.Fatalf("forgotten jobs came back: %+v", list)
	}
}

func TestRenameMovesTheJobsOfAnOwnerWhileTheyRun(t *testing.T) {
	e := newEnv(t)
	release := make(chan struct{})
	owner := make(chan string, 1)
	wait := gated(release)
	running := e.submit(t, "test", addScript(t, func(ctx context.Context, run *Run) error {
		err := wait(ctx, run)
		owner <- run.Owner()
		return err
	}), 1)
	e.waitState(t, running.ID, Running)
	if err := e.m.Rename("anna", "anni"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if got := <-owner; got != "anni" {
		t.Errorf("the running job goes on for %q", got)
	}
	e.waitState(t, running.ID, Done)
	e.restart(t)
	if list := e.m.List("anni"); len(list) != 1 || list[0].ID != running.ID {
		t.Errorf("after a restart: %+v", list)
	}
}

func TestEventsOnlyReachTheOwner(t *testing.T) {
	e := newEnv(t)
	other, _, _ := e.hub.Connect("tab-bert", "bert", 0)
	release := make(chan struct{})
	close(release)
	j := e.submit(t, "test", addScript(t, gated(release)), 1)
	e.waitState(t, j.ID, Done)
	select {
	case ev := <-other.Events:
		t.Fatalf("another user got %s", ev.Type)
	default:
	}
}

func TestPanicInAKindFailsTheJob(t *testing.T) {
	e := newEnv(t)
	j := e.submit(t, "test", addScript(t, func(context.Context, *Run) error { panic("boom") }), 1)
	if failed := e.waitState(t, j.ID, Failed); failed.Error == "" {
		t.Fatal("no error text")
	}
}

func TestJobJSONHasTheFieldsTheClientReads(t *testing.T) {
	left := 3.5
	raw, err := json.Marshal(Job{ID: "x", State: Queued, Left: &left, CreatedAt: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "state", "left", "doneBytes", "totalBytes", "createdAt"} {
		if _, ok := m[k]; !ok {
			t.Errorf("%s is missing", k)
		}
	}
	if _, ok := m["finishedAt"]; ok {
		t.Error("finishedAt is set on an open job")
	}
}

func TestManyItemsAreReportedAtMostFourTimesASecond(t *testing.T) {
	e := newEnv(t)
	client, _, _ := e.hub.Connect("tab", "anna", 0)
	progress := make(chan int)
	go func() {
		n := 0
		for ev := range client.Events {
			if ev.Type == EventProgress {
				n++
			}
		}
		progress <- n
	}()
	name := addScript(t, func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			run.Begin(it.Seq)
			run.Finish(it.Seq, ItemDone, nil)
		}
		return nil
	})
	began := time.Now()
	j := e.submit(t, "test", name, 2000)
	e.waitState(t, j.ID, Done)
	took := time.Since(began)
	e.hub.Disconnect(client)
	if n, most := <-progress, int(took/progressEvery)+2; n > most {
		t.Fatalf("%d progress events in %s, at most %d expected", n, took, most)
	}
}

func BenchmarkItemOfAJobWithFiftyThousandItems(b *testing.B) {
	m, err := New(v1.Deps{DB: testutil.MainDB(b)}, Options{})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(m.Close)
	j := &job{}
	j.ID = "big"
	for i := range 50000 {
		j.items = append(j.items, Item{Seq: i, Path: "/mnt/user/x", State: ItemPending, Bytes: 1, Files: 1})
	}
	run := &Run{m: m, j: j}
	seq := 0
	for b.Loop() {
		run.Begin(seq)
		run.Measure(seq, 1, 1)
		run.Finish(seq, ItemDone, nil)
		seq = (seq + 1) % len(j.items)
	}
}

func TestAFinishedItemIsReportedWhileTheNextOneTakesLong(t *testing.T) {
	e := newEnv(t)
	client, _, _ := e.hub.Connect("tab", "anna", 0)
	release := make(chan struct{})
	name := addScript(t, func(ctx context.Context, run *Run) error {
		items := run.Items()
		run.Measure(items[0].Seq, 1, 1)
		run.Finish(items[0].Seq, ItemDone, nil)
		run.Begin(items[1].Seq)
		<-release
		run.Finish(items[1].Seq, ItemDone, nil)
		return nil
	})
	defer close(release)
	e.submit(t, "test", name, 2)
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-client.Events:
			var p Progress
			if ev.Type == EventProgress && json.Unmarshal(ev.Data, &p) == nil && p.DoneFiles == 1 {
				return
			}
		case <-timeout:
			t.Fatal("the finished item was never reported")
		}
	}
}

func TestABigResultIsOnlyReadFromTheJobItself(t *testing.T) {
	e := newEnv(t)
	client, _, _ := e.hub.Connect("tab", "anna", 0)
	p := &Plan{Params: struct{}{}, Items: []Item{{Path: "/mnt/user/x"}}}
	j, err := e.m.Submit("anna", "bigresult", p, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	e.waitState(t, j.ID, Done)
	for _, listed := range e.m.List("") {
		if listed.Result != nil {
			t.Errorf("the list holds the result %v", listed.Result)
		}
	}
	for len(client.Events) > 0 {
		if ev := <-client.Events; strings.Contains(string(ev.Data), `"result":`) {
			t.Errorf("%s holds the result: %s", ev.Type, ev.Data)
		}
	}
	if full, _ := e.m.Get(j.ID, true); full.Result == nil {
		t.Error("the job lost its result")
	}
	e.restart(t)
	if full, _ := e.m.Get(j.ID, true); full.Result == nil {
		t.Error("the job lost its result in the restart")
	}
}

func TestAFinishedBigJobKeepsItsItemsOnlyInTheDatabase(t *testing.T) {
	e := newEnv(t)
	name := addScript(t, func(ctx context.Context, run *Run) error {
		for _, it := range run.Items() {
			run.Measure(it.Seq, 2, 1)
			run.Finish(it.Seq, ItemDone, nil)
		}
		return nil
	})
	n := maxItemsKept + 1
	j := e.submit(t, "test", name, n)
	e.waitState(t, j.ID, Done)
	check := func() {
		t.Helper()
		e.m.mu.Lock()
		held := len(e.m.jobs[j.ID].items)
		e.m.mu.Unlock()
		if held != 0 {
			t.Errorf("%d items in memory", held)
		}
		got, _ := e.m.Get(j.ID, true)
		if len(got.Items) != n || got.Items[n-1].Seq != n-1 || got.Items[n-1].State != ItemDone || got.Items[n-1].Bytes != 2 {
			t.Fatalf("%d items, the last one %+v", len(got.Items), got.Items[len(got.Items)-1])
		}
		if got.Count != n || got.TotalFiles != int64(n) || got.DoneBytes != int64(2*n) {
			t.Errorf("totals of the job = %+v", got)
		}
	}
	check()
	e.restart(t)
	check()
}
