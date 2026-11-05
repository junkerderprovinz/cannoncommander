package fileops_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// array is a small Unraid with two data disks below a temp dir.
type array struct {
	t   *testing.T
	mnt string
}

func newArray(t *testing.T) *array {
	t.Helper()
	base := t.TempDir()
	cfg := unraid.DefaultConfig()
	cfg.Emhttp = filepath.Join(base, "emhttp")
	cfg.Shares = filepath.Join(base, "shares")
	cfg.BootConfig = filepath.Join(base, "config")
	cfg.Mdstat = filepath.Join(base, "mdstat")
	cfg.Mnt = filepath.Join(base, "mnt")
	cfg.Notifications = filepath.Join(base, "notifications")
	cfg.Socket = filepath.Join(base, "emhttpd.socket")
	a := &array{t: t, mnt: cfg.Mnt}

	var disks strings.Builder
	for _, d := range []string{"disk1", "disk2"} {
		fmt.Fprintf(&disks, "[\"%s\"]\nname=\"%s\"\ntype=\"Data\"\nstatus=\"DISK_OK\"\nfsStatus=\"Mounted\"\nfsMountpoint=\"/mnt/%s\"\nsize=\"4096\"\nfsSize=\"4096\"\nfsFree=\"2048\"\nspundown=\"0\"\n", d, d, d)
		if err := os.MkdirAll(filepath.Join(cfg.Mnt, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a.write(filepath.Join(cfg.Emhttp, "disks.ini"), disks.String())
	a.write(filepath.Join(cfg.Emhttp, "shares.ini"), "[\"filme\"]\nname=\"filme\"\nuseCache=\"no\"\ncachePool=\"\"\nallocator=\"highwater\"\nfloor=\"0\"\nexclude=\"\"\nsplitLevel=\"\"\nused=\"7\"\n")
	a.write(filepath.Join(cfg.Emhttp, "var.ini"), "version=\"7.3.4\"\nNAME=\"Tower\"\nmdState=\"STARTED\"\n")
	a.write(cfg.Mdstat, "mdState=STARTED\nmdResync=0\nmdResyncPos=0\n")
	t.Cleanup(fileops.UseUnraid(unraid.New(cfg)))
	return a
}

func (a *array) write(p, content string) {
	a.t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		a.t.Fatal(err)
	}
}

func (a *array) path(dev, rel string) string {
	return filepath.Join(a.mnt, dev, filepath.FromSlash(rel))
}

func (a *array) has(dev, rel string) bool {
	_, err := os.Lstat(a.path(dev, rel))
	return err == nil
}

func waitAs(e *env, user, id string) jobs.Job {
	e.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var j jobs.Job
		e.call(user, "GET", "/jobs/"+id, nil, &j)
		if !j.State.Open() {
			return j
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("job %s stuck in %s", id, j.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestGatherMovesAFolderOntoOneDisk(t *testing.T) {
	e := newEnv(t)
	a := newArray(t)
	a.write(a.path("disk1", "filme/Sintel/Sintel.mkv"), "sintel")
	a.write(a.path("disk1", "filme/Sintel/extras/poster.jpg"), "poster")
	a.write(a.path("disk2", "filme/Big Buck Bunny/bbb.mkv"), "bunny")

	j := gather(e, "disk2")
	for _, rel := range []string{"filme/Sintel/Sintel.mkv", "filme/Sintel/extras/poster.jpg", "filme/Big Buck Bunny/bbb.mkv"} {
		if !a.has("disk2", rel) {
			t.Errorf("%s is not on disk2", rel)
		}
	}
	if b, _ := os.ReadFile(a.path("disk2", "filme/Sintel/Sintel.mkv")); string(b) != "sintel" {
		t.Errorf("content = %q", b)
	}
	if a.has("disk1", "filme/Sintel") {
		t.Error("the emptied folder stays on disk1")
	}
	if !a.has("disk1", "filme") {
		t.Error("the share folder itself must stay")
	}
	if res, _ := j.Result.(map[string]any); res["moved"] != float64(2) {
		t.Errorf("result = %v", j.Result)
	}
}

func TestAGatherStopsWhenItsOwnerIsNoAdminAnymore(t *testing.T) {
	e := newEnv(t)
	a := newArray(t)
	a.write(a.path("disk1", "filme/Sintel/Sintel.mkv"), "sintel")
	e.demoted.Store(true)
	params := map[string]any{"mode": unraid.ModeGather, "path": "/mnt/user/filme", "disk": "disk2"}
	j, err := jobs.For(e.deps).Submit("admin", unraid.ModeGather, &jobs.Plan{Params: params, Items: []jobs.Item{{Path: "/mnt/user/filme", Name: "filme"}}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	waitAs(e, "admin", j.ID)
	if !a.has("disk1", "filme/Sintel/Sintel.mkv") || a.has("disk2", "filme/Sintel/Sintel.mkv") {
		t.Error("the job went on")
	}
}

func gather(e *env, disk string) jobs.Job {
	e.t.Helper()
	var j jobs.Job
	body := map[string]any{"kind": unraid.ModeGather, "params": map[string]any{"path": "/mnt/user/filme", "disk": disk}}
	if code := e.call("admin", "POST", "/jobs", body, &j); code != http.StatusAccepted {
		e.t.Fatalf("start: %d", code)
	}
	j = waitAs(e, "admin", j.ID)
	if j.State != jobs.Done {
		e.t.Fatalf("job ended %s: %+v", j.State, j.Items)
	}
	return j
}

func TestOnlyAdminsGatherAndOnlyOntoAKnownDisk(t *testing.T) {
	e := newEnv(t)
	newArray(t)
	params := map[string]any{"path": "/mnt/user/filme", "disk": "disk2"}
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": unraid.ModeGather, "params": params}, nil); code != http.StatusForbidden {
		t.Errorf("anna: %d", code)
	}
	params["disk"] = "disk9"
	if code := e.call("admin", "POST", "/jobs", map[string]any{"kind": unraid.ModeScatter, "params": params}, nil); code != http.StatusBadRequest {
		t.Errorf("unknown disk: %d", code)
	}
	params["disk"], params["path"] = "disk1", "/mnt/disk1/filme"
	if code := e.call("admin", "POST", "/jobs", map[string]any{"kind": unraid.ModeGather, "params": params}, nil); code != http.StatusBadRequest {
		t.Errorf("disk path: %d", code)
	}
}
