package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

const kindHold = "testHold"

var (
	holdOnce sync.Once
	holdEnds = make(chan chan struct{}, 1)
)

// holdQueue keeps the only slot of the queue busy, so the next job stays queued
// until the returned func runs.
func holdQueue(e *env) func() {
	e.t.Helper()
	holdOnce.Do(func() {
		jobs.RegisterKind(kindHold, jobs.Kind{
			Prepare: func(context.Context, v1.Deps, *users.User, json.RawMessage) (*jobs.Plan, error) {
				return &jobs.Plan{Items: []jobs.Item{{Name: "halten"}}}, nil
			},
			Run: func(ctx context.Context, run *jobs.Run) error {
				select {
				case <-<-holdEnds:
				case <-ctx.Done():
				}
				return nil
			},
		})
	})
	end := make(chan struct{})
	holdEnds <- end
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": kindHold, "params": map[string]any{}}, nil); code != http.StatusAccepted {
		e.t.Fatalf("hold: %d", code)
	}
	return func() { close(end) }
}

func waitFor(e *env, id string) jobs.Job {
	e.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var j jobs.Job
		e.call("anna", "GET", "/jobs/"+id, nil, &j)
		if !j.State.Open() {
			return j
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("job %s stuck in %s", id, j.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAVerifyJobAsksForTheRightsWhenItRuns(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/geheim/a.txt", "a")
	release := holdQueue(e)
	var j jobs.Job
	sums := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c685c9c5dc0d6a0ac0ad4  /mnt/user/fotos/geheim/a.txt\n"
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": KindVerify, "params": VerifyParams{Text: sums, Dir: "/mnt/user/fotos"}}, &j); code != http.StatusAccepted {
		t.Fatalf("verify: %d", code)
	}
	e.rules.set("/mnt/user/fotos/geheim", ccfs.Hidden)
	release()
	done := waitFor(e, j.ID)
	raw, _ := json.Marshal(done.Result)
	var out struct {
		Lines []Checked `json:"lines"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Lines) != 1 || out.Lines[0].State == "ok" || out.Lines[0].State == "mismatch" {
		t.Errorf("the job read what is hidden by now: %s", raw)
	}
}

func TestATestArchiveJobAsksForTheRightsWhenItRuns(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/geheim/x.txt", "x")
	e.run(KindPack, PackParams{Sources: []string{"/mnt/user/fotos/geheim/x.txt"}, Dest: "/mnt/user/fotos/geheim", Name: "a", Format: "zip"})
	release := holdQueue(e)
	var j jobs.Job
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": KindTestArchive, "params": TestArchiveParams{Paths: []string{"/mnt/user/fotos/geheim/a.zip"}}}, &j); code != http.StatusAccepted {
		t.Fatalf("test: %d", code)
	}
	e.rules.set("/mnt/user/fotos/geheim", ccfs.Hidden)
	release()
	done := waitFor(e, j.ID)
	if len(done.Items) != 1 || done.Items[0].State != jobs.ItemFailed {
		t.Errorf("the job read what is hidden by now: %+v", done.Items)
	}
}
