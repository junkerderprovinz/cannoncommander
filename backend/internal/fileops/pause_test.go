package fileops_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// slowCopy starts a copy that takes seconds and waits for its first bytes.
func slowCopy(e *env, from string) jobs.Job {
	e.t.Helper()
	var j jobs.Job
	code := e.call("anna", "POST", "/jobs", map[string]any{"kind": fileops.KindCopy, "cap": 1 << 20, "params": map[string]any{"sources": []string{from}, "dest": "/mnt/user/backups"}}, &j)
	if code != http.StatusAccepted {
		e.t.Fatalf("start: %d", code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for j.DoneBytes == 0 {
		if time.Now().After(deadline) {
			e.t.Fatal("no bytes moved")
		}
		time.Sleep(5 * time.Millisecond)
		e.call("anna", "GET", "/jobs/"+j.ID, nil, &j)
	}
	return j
}

func TestAPausedCopyAsksForTheRightsAgain(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/gross.bin", strings.Repeat("x", 4<<20), time.Now())
	j := slowCopy(e, "/mnt/user/fotos/gross.bin")
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/pause", nil, nil); code != http.StatusOK {
		t.Fatalf("pause: %d", code)
	}
	e.rules.set("/mnt/user/fotos", ccfs.Hidden)
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/resume", nil, nil); code != http.StatusOK {
		t.Fatalf("resume: %d", code)
	}
	e.wait(j.ID)
	if !e.gone("/mnt/user/backups/gross.bin") {
		t.Error("the copy finished on rights that are gone")
	}
	if !e.gone("/mnt/user/backups/.gross.bin.cc-part") {
		t.Error("the part file stayed")
	}
}

func TestACopyDoesNotCountWhatIsHiddenInside(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.Hidden)
	j := e.run(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/album"}, "dest": "/mnt/user/backups"})
	if j.TotalBytes != int64(len("frei")) || j.TotalFiles != 2 {
		t.Errorf("%d bytes in %d entries", j.TotalBytes, j.TotalFiles)
	}
}

func TestPreflightDoesNotCountWhatIsHiddenInside(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.Hidden)
	var out struct {
		Bytes int64 `json:"bytes"`
		Files int64 `json:"files"`
	}
	code := e.call("anna", "POST", "/ops/preflight", map[string]any{"kind": "copy", "sources": []string{"/mnt/user/fotos/album"}, "dest": "/mnt/user/backups"}, &out)
	if code != http.StatusOK {
		t.Fatalf("%d", code)
	}
	if out.Bytes != int64(len("frei")) || out.Files != 2 {
		t.Errorf("%d bytes in %d entries", out.Bytes, out.Files)
	}
}
