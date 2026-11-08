//go:build unix

package fileops_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
)

type window struct{ desktop.App }

// denied gives a folder no write rights, which the job meets as a refusal
// of the system, and returns the function that gives them back, standing in
// for the rights the helper brings.
func denied(t *testing.T, e *env, api string) func() {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root is never refused")
	}
	dir := e.real(api)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o777) })
	return func() { os.Chmod(dir, 0o777) }
}

// elevated starts the desktop app's way of going on with administrator
// rights, with the helper in this process.
func elevated(t *testing.T) {
	t.Cleanup(elevate.InProcess())
	desktop.Use(window{}, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
}

func (e *env) goOnElevated(j jobs.Job) jobs.Job {
	e.t.Helper()
	if j.State != jobs.Failed || j.Items[0].ErrorCode != "denied" {
		e.t.Fatalf("the job did not fail for lack of rights: %+v", j)
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/elevate", nil, nil); code != http.StatusOK {
		e.t.Fatalf("elevate: %d", code)
	}
	return e.ended(j.ID)
}

func TestADeniedCopyGoesOnWithAdministratorRights(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	grant := denied(t, e, "/mnt/user/backups")
	j := e.wait(e.start("copy", map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups"}).ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
	if e.read("/mnt/user/backups/a.jpg") != "a" || e.read("/mnt/user/fotos/a.jpg") != "a" {
		t.Fatal("not copied")
	}
}

func TestADeniedMoveKeepsBothWhenAsked(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/a.jpg", "neu", time.Time{})
	e.write("/mnt/user/backups/a.jpg", "alt", time.Time{})
	grant := denied(t, e, "/mnt/user/fotos")
	j := e.wait(e.start("move", map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups", "rule": "both"}).ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
	if e.read("/mnt/user/backups/a.jpg") != "alt" || e.read("/mnt/user/backups/a (1).jpg") != "neu" || !e.gone("/mnt/user/fotos/a.jpg") {
		t.Fatal("not moved beside the old file")
	}
}

func TestADeniedDeleteGoesOnWithAdministratorRights(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	grant := denied(t, e, "/mnt/user/fotos")
	j := e.wait(e.start("delete", map[string]any{"paths": []string{"/mnt/user/fotos/a.jpg"}}).ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done || !e.gone("/mnt/user/fotos/a.jpg") {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
}

func TestAHelperWithoutTheRightsLeavesTheItemsFailed(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	denied(t, e, "/mnt/user/fotos")
	j := e.wait(e.start("delete", map[string]any{"paths": []string{"/mnt/user/fotos/a.jpg"}}).ID)
	j = e.goOnElevated(j)
	if j.State != jobs.Failed || j.Items[0].ErrorCode != "denied" || e.gone("/mnt/user/fotos/a.jpg") {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
}
