package fileops_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// linkedFotos makes /mnt/user/backups/tor a link to /mnt/user/fotos.
func linkedFotos(e *env) {
	e.t.Helper()
	linksOnly(e.t)
	if err := os.Symlink(e.real("/mnt/user/fotos"), e.real("/mnt/user/backups/tor")); err != nil {
		e.t.Fatal(err)
	}
}

func TestACopyThroughALinkFolderGoesByTheRulesWhereItLeads(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.Hidden)
	linkedFotos(e)
	e.run(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/backups/tor/album"}, "dest": "/mnt/user/backups"})
	if e.gone("/mnt/user/backups/album/frei.jpg") {
		t.Error("the open file is missing")
	}
	if !e.gone("/mnt/user/backups/album/geheim/x.jpg") {
		t.Error("the hidden file came along")
	}
}

func TestAFolderTakenThroughALinkStillAnswersForTheRulesInside(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.ReadOnly)
	linkedFotos(e)
	album := "/mnt/user/backups/tor/album"
	if code := e.startAnswer(fileops.KindDelete, map[string]any{"paths": []string{album}}); code != http.StatusForbidden {
		t.Errorf("delete: %d", code)
	}
	if code := e.startAnswer(trash.KindTrash, map[string]any{"paths": []string{album}}); code != http.StatusForbidden {
		t.Errorf("trash: %d", code)
	}
	if code := e.startAnswer(fileops.KindMove, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups"}); code != http.StatusForbidden {
		t.Errorf("move: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": album, "name": "anders"}, nil); code != http.StatusForbidden {
		t.Errorf("rename: %d", code)
	}
}

func TestACopyIntoALinkedFolderDoesNotReplaceAReadOnlyFile(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/fest.jpg", "alt", old)
	e.write("/mnt/user/backups/fest.jpg", "neu", old)
	e.rules.set("/mnt/user/fotos/fest.jpg", ccfs.ReadOnly)
	linkedFotos(e)
	code := e.call("anna", "POST", "/jobs", map[string]any{"kind": fileops.KindCopy, "params": map[string]any{
		"sources": []string{"/mnt/user/backups/fest.jpg"}, "dest": "/mnt/user/backups/tor", "rule": jobs.Replace}}, nil)
	if code < 300 {
		time.Sleep(200 * time.Millisecond)
	}
	if got := e.read("/mnt/user/fotos/fest.jpg"); got != "alt" {
		t.Errorf("the read only file changed: %q", got)
	}
}

func TestALinkItselfCanBeTakenAwayWhereverItLeads(t *testing.T) {
	linksOnly(t)
	for name, target := range map[string]string{"dangling": "/nirgends/da", "outside": os.TempDir()} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			at := "/mnt/user/fotos/tor"
			if err := os.Symlink(target, e.real(at)); err != nil {
				t.Fatal(err)
			}
			e.run(fileops.KindDelete, map[string]any{"paths": []string{at}})
			if !e.gone(at) {
				t.Error("delete left the link")
			}
			if err := os.Symlink(target, e.real(at)); err != nil {
				t.Fatal(err)
			}
			e.run(trash.KindTrash, map[string]any{"paths": []string{at}})
			if !e.gone(at) {
				t.Error("trash left the link")
			}
			if err := os.Symlink(target, e.real(at)); err != nil {
				t.Fatal(err)
			}
			if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": at, "name": "anders"}, nil); code != http.StatusOK {
				t.Errorf("rename: %d", code)
			}
		})
	}
}

func TestDeletingALinkToAHiddenPlaceLeavesThePlace(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.Hidden)
	linkedFotos(e)
	if err := os.Symlink(e.real("/mnt/user/fotos/album/geheim"), e.real("/mnt/user/backups/tor2")); err != nil {
		t.Fatal(err)
	}
	e.run(fileops.KindDelete, map[string]any{"paths": []string{"/mnt/user/backups/tor2"}})
	if !e.gone("/mnt/user/backups/tor2") {
		t.Error("the link stayed")
	}
	if e.gone("/mnt/user/fotos/album/geheim/x.jpg") {
		t.Error("the file behind the link went away")
	}
}

func TestUndoOfARenameAnswersForRulesInsideTheFolder(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a/x.jpg", "x", time.Now())
	var out fileops.Changed
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/a", "name": "b"}, &out); code != http.StatusOK {
		t.Fatalf("rename: %d", code)
	}
	e.rules.set("/mnt/user/fotos/b/x.jpg", ccfs.ReadOnly)
	e.call("anna", "POST", "/undo", map[string]string{"id": out.Undo}, nil)
	if e.gone("/mnt/user/fotos/b/x.jpg") {
		t.Error("undo took the file out from under its rule")
	}
}

func TestTheSweepLeavesThePartFileOfAnOpenJob(t *testing.T) {
	e := newEnv(t)
	long := time.Now().Add(-48 * time.Hour)
	const part = "/mnt/user/backups/.a.jpg.cc-part"
	e.write(part, "x", long)
	e.write("/mnt/user/fotos/a.jpg", "a", long)
	e.write("/mnt/user/backups/a.jpg", "alt", long)
	j := e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups"})
	if got := e.wait(j.ID); got.State != jobs.Asking {
		t.Fatalf("state %s", got.State)
	}
	sweepNow(e)
	if e.gone(part) {
		t.Error("the sweep took the part file of a job that waits")
	}
}

func TestTheSweepOnlyTakesPartFilesByTheirExactName(t *testing.T) {
	e := newEnv(t)
	long := time.Now().Add(-48 * time.Hour)
	const other = "/mnt/user/fotos/.foto.CC-PART"
	e.write(other, "x", long)
	sweepNow(e)
	if e.gone(other) {
		t.Error("the sweep took a file of the user")
	}
}
