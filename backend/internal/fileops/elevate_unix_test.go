//go:build desktop && unix

package fileops_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
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

func TestADeniedCopyLeavesWhatIsHiddenBehindAsANormalCopyDoes(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	album := albumWithThumbs(e)
	e.write("/mnt/user/fotos/album/.bild.jpg.cc-part", "halb", time.Time{})
	grant := denied(t, e, "/mnt/user/backups")
	j := e.wait(e.start("copy", map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups"}).ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
	if e.read("/mnt/user/backups/album/bild.jpg") != "bild" {
		t.Fatal("not copied")
	}
	for _, p := range []string{"album/Thumbs.db", "album/sub/Thumbs.db", "album/.bild.jpg.cc-part"} {
		if !e.gone("/mnt/user/backups/" + p) {
			t.Errorf("%s was copied", p)
		}
	}
}

func TestADeniedMoveLeavesPartFilesBehind(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/album/bild.jpg", "bild", time.Time{})
	e.write("/mnt/user/fotos/album/.bild.jpg.cc-part", "halb", time.Time{})
	grant := denied(t, e, "/mnt/user/backups")
	j := e.wait(e.start("move", map[string]any{"sources": []string{"/mnt/user/fotos/album"}, "dest": "/mnt/user/backups"}).ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
	if e.read("/mnt/user/backups/album/bild.jpg") != "bild" || !e.gone("/mnt/user/fotos/album/bild.jpg") {
		t.Fatal("not moved")
	}
	if !e.gone("/mnt/user/backups/album/.bild.jpg.cc-part") || e.read("/mnt/user/fotos/album/.bild.jpg.cc-part") != "halb" {
		t.Fatal("the part file went along")
	}
}

func TestADeniedMoveIntoANewFolderMakesTheFolderElevated(t *testing.T) {
	e := newEnv(t)
	elevated(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	grant := denied(t, e, "/mnt/user/backups")
	j := e.wait(e.start("move", map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups/neu", "newFolder": true}).ID)
	grant()
	if j.State != jobs.Failed || j.ErrorCode != "denied" {
		t.Fatalf("the job did not fail for lack of rights: %+v", j)
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/elevate", nil, nil); code != http.StatusOK {
		t.Fatalf("elevate: %d", code)
	}
	if j = e.ended(j.ID); j.State != jobs.Done {
		t.Fatalf("ended %s: %s %+v", j.State, j.Error, j.Items)
	}
	if e.read("/mnt/user/backups/neu/a.jpg") != "a" || !e.gone("/mnt/user/fotos/a.jpg") {
		t.Fatal("not moved into the new folder")
	}
}

// inSystemTrash is the desktop app with the trash of the system, here the
// freedesktop.org one in a home folder of the test's own.
func inSystemTrash(t *testing.T) *env {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	elevated(t)
	return newEnv(t)
}

func TestARestoreIntoAProtectedFolderGoesOnElevated(t *testing.T) {
	e := inSystemTrash(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	e.run(trash.KindTrash, map[string]any{"paths": []string{"/mnt/user/fotos/a.jpg"}})
	items := e.trashItems()
	if len(items) != 1 || !strings.Contains(items[0].ID, ".") {
		t.Fatalf("not in the system trash: %+v", items)
	}
	grant := denied(t, e, "/mnt/user/fotos")
	var j jobs.Job
	if code := e.call("anna", "POST", "/trash/restore", map[string]any{"ids": []string{items[0].ID}}, &j); code != http.StatusAccepted {
		t.Fatalf("restore: %d", code)
	}
	j = e.wait(j.ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %+v", j.State, j.Items)
	}
	if e.read("/mnt/user/fotos/a.jpg") != "a" || len(e.trashItems()) != 0 {
		t.Fatal("not restored")
	}
}

func TestTakingBackATrashIntoAProtectedFolderGoesOnElevated(t *testing.T) {
	e := inSystemTrash(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Time{})
	e.write("/mnt/user/fotos/b.jpg", "b", time.Time{})
	trashed := e.run(trash.KindTrash, map[string]any{"paths": []string{"/mnt/user/fotos/a.jpg", "/mnt/user/fotos/b.jpg"}})
	grant := denied(t, e, "/mnt/user/fotos")
	var out struct {
		Job *jobs.Job `json:"job"`
	}
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": trashed.Undo}, &out); code != http.StatusAccepted || out.Job == nil {
		t.Fatalf("undo: %d", code)
	}
	j := e.wait(out.Job.ID)
	grant()
	if j = e.goOnElevated(j); j.State != jobs.Done {
		t.Fatalf("ended %s: %s %+v", j.State, j.Error, j.Items)
	}
	if e.read("/mnt/user/fotos/a.jpg") != "a" || e.read("/mnt/user/fotos/b.jpg") != "b" || len(e.trashItems()) != 0 {
		t.Fatal("not taken back")
	}
}
