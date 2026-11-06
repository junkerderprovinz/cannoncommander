package fileops_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// arrayEnv serves an array with two disks next to the user shares, and
// returns the resolver so a test can take a disk away.
func arrayEnv(t *testing.T) (*env, *array, *ccfs.Resolver) {
	t.Helper()
	a := newArray(t)
	var fsys *ccfs.Resolver
	e := newEnvWith(t, func(r *ccfs.Resolver) *ccfs.Resolver {
		fsys = r.WithDevices(a.mnt, "disk1", "disk2")
		return fsys
	})
	return e, a, fsys
}

func startAsAdmin(e *env, kind string, params any, cap int) jobs.Job {
	e.t.Helper()
	var j jobs.Job
	body := map[string]any{"kind": kind, "params": params}
	if cap > 0 {
		body["cap"] = cap
	}
	if code := e.call("admin", "POST", "/jobs", body, &j); code != http.StatusAccepted {
		e.t.Fatalf("start %s: %d", kind, code)
	}
	return j
}

func TestACopyDoesNotFinishWhenItsDiskGoesAwayOnTheWay(t *testing.T) {
	e, a, fsys := arrayEnv(t)
	a.write(a.path("disk1", "filme/Sintel/a.mkv"), "a")
	a.write(a.path("disk1", "filme/Sintel/b.mkv"), "b")
	stop := fschange.Observe(fschange.Observer{Created: func(p string) {
		if p == "/mnt/user/backups/Sintel/a.mkv" {
			fsys.SetDevices("disk2")
		}
	}})
	defer stop()

	j := startAsAdmin(e, fileops.KindCopy, fileops.TransferParams{Sources: []string{"/mnt/disk1/filme/Sintel"}, Dest: "/mnt/user/backups"}, 0)
	j = waitAs(e, "admin", j.ID)
	if j.State == jobs.Done || len(j.Items) != 1 || j.Items[0].State != jobs.ItemFailed {
		t.Fatalf("job %s with items %+v", j.State, j.Items)
	}
	if !e.gone("/mnt/user/backups/Sintel/b.mkv") || !a.has("disk1", "filme/Sintel/b.mkv") {
		t.Fatal("the file behind the missing disk moved")
	}
}

func TestAPausedCopyFromADiskThatWentAwayLeavesNoHalfFile(t *testing.T) {
	e, a, fsys := arrayEnv(t)
	a.write(a.path("disk1", "filme/gross.mkv"), strings.Repeat("x", 4<<20))
	j := startAsAdmin(e, fileops.KindCopy, fileops.TransferParams{Sources: []string{"/mnt/disk1/filme/gross.mkv"}, Dest: "/mnt/user/backups"}, 1<<20)
	deadline := time.Now().Add(10 * time.Second)
	for j.DoneBytes == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no bytes moved")
		}
		time.Sleep(5 * time.Millisecond)
		e.call("admin", "GET", "/jobs/"+j.ID, nil, &j)
	}
	if code := e.call("admin", "POST", "/jobs/"+j.ID+"/pause", nil, nil); code != http.StatusOK {
		t.Fatalf("pause: %d", code)
	}
	fsys.SetDevices("disk2")
	if code := e.call("admin", "POST", "/jobs/"+j.ID+"/resume", nil, nil); code != http.StatusOK {
		t.Fatalf("resume: %d", code)
	}
	if j = waitAs(e, "admin", j.ID); j.State == jobs.Done {
		t.Fatal("the copy finished from a disk that is gone")
	}
	if !a.has("disk1", "filme/gross.mkv") {
		t.Fatal("the source is gone")
	}
	if !e.gone("/mnt/user/backups/gross.mkv") || !e.gone("/mnt/user/backups/.gross.mkv.cc-part") {
		t.Fatal("a half copied file stayed at the target")
	}
}
