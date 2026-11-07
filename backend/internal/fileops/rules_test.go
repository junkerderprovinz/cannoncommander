package fileops_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex/fsindextest"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

func sweepNow(e *env) {
	e.t.Helper()
	db := fsindextest.New(e.t)
	fsindextest.Scan(e.t, db, "Shares", e.root)
	fileops.For(e.deps).SweepParts(e.t.Context())
}

func linksOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks")
	}
}

// startAnswer posts a job and reports the status it was answered with.
func (e *env) startAnswer(kind string, params any) int {
	e.t.Helper()
	return e.call("anna", "POST", "/jobs", map[string]any{"kind": kind, "params": params}, nil)
}

func protectedFolder(e *env, effect ccfs.Effect) {
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/album/frei.jpg", "frei", old)
	e.write("/mnt/user/fotos/album/geheim/x.jpg", "geheim", old)
	e.rules.set("/mnt/user/fotos/album/geheim", effect)
}

func TestAFolderWithAProtectedEntryCannotBeTakenAway(t *testing.T) {
	for name, effect := range map[string]ccfs.Effect{"hidden": ccfs.Hidden, "read only": ccfs.ReadOnly} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			protectedFolder(e, effect)
			album := "/mnt/user/fotos/album"
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
			time.Sleep(50 * time.Millisecond)
			if e.gone("/mnt/user/fotos/album/geheim/x.jpg") {
				t.Error("the protected file went away")
			}
		})
	}
}

func TestAnAnswerAsksForTheRightsAgain(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/geheim/x.jpg", "geheim", old)
	e.write("/mnt/user/backups/x.jpg", "alt", old)
	asking := e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/geheim/x.jpg"}, "dest": "/mnt/user/backups"})
	if got := e.wait(asking.ID); got.State != jobs.Asking {
		t.Fatalf("state %s", got.State)
	}
	e.rules.set("/mnt/user/fotos/geheim", ccfs.Hidden)
	if code := e.call("anna", "POST", "/jobs/"+asking.ID+"/decide", map[string]string{"key": "x.jpg", "choice": jobs.Replace}, nil); code != http.StatusOK {
		t.Fatalf("decide: %d", code)
	}
	e.wait(asking.ID)
	if got := e.read("/mnt/user/backups/x.jpg"); got != "alt" {
		t.Errorf("a hidden source was copied after the answer: %q", got)
	}
}

func TestARetryAsksForTheRightsAgain(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/a.jpg", "a", old)
	e.write("/mnt/user/backups/a.jpg", "alt", old)
	j := e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups"})
	if got := e.wait(j.ID); got.State != jobs.Asking {
		t.Fatalf("state %s", got.State)
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/cancel", nil, nil); code != http.StatusOK {
		t.Fatalf("cancel: %d", code)
	}
	e.ended(j.ID)
	e.rules.set("/mnt/user/fotos", ccfs.Hidden)
	if err := os.Remove(e.real("/mnt/user/backups/a.jpg")); err != nil {
		t.Fatal(err)
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/retry", nil, nil); code != http.StatusOK {
		t.Fatalf("retry: %d", code)
	}
	e.wait(j.ID)
	if !e.gone("/mnt/user/backups/a.jpg") {
		t.Error("a retry copied what is hidden by now")
	}
}

func TestACopyLeavesOutWhatIsHiddenInside(t *testing.T) {
	e := newEnv(t)
	protectedFolder(e, ccfs.Hidden)
	e.run(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/album"}, "dest": "/mnt/user/backups"})
	if e.gone("/mnt/user/backups/album/frei.jpg") {
		t.Error("the open file is missing")
	}
	if !e.gone("/mnt/user/backups/album/geheim/x.jpg") {
		t.Error("the hidden file came along")
	}
}

func TestACopyDoesNotChangeAReadOnlyPlace(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	for name, rule := range map[string]string{"replacing it": jobs.Replace, "merging into it": jobs.Newer} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.write("/mnt/user/fotos/ziel/fest/neu.jpg", "neu", old)
			e.write("/mnt/user/backups/ziel/fest/alt.jpg", "alt", old)
			e.rules.set("/mnt/user/backups/ziel/fest", ccfs.ReadOnly)
			e.wait(e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/ziel"}, "dest": "/mnt/user/backups", "rule": rule}).ID)
			if !e.gone("/mnt/user/backups/ziel/fest/neu.jpg") || e.gone("/mnt/user/backups/ziel/fest/alt.jpg") {
				t.Error("the read only folder changed")
			}
		})
	}
}

func TestACopyDoesNotReplaceAFolderWithAReadOnlyEntry(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/ziel", "datei statt ordner", old)
	e.write("/mnt/user/backups/ziel/fest/alt.jpg", "alt", old)
	e.rules.set("/mnt/user/backups/ziel/fest", ccfs.ReadOnly)
	e.wait(e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/ziel"}, "dest": "/mnt/user/backups", "rule": jobs.Replace}).ID)
	if _, err := os.Stat(e.real("/mnt/user/backups/ziel/fest/alt.jpg")); err != nil {
		t.Errorf("the read only entry went away with its folder: %v", err)
	}
}

func TestUndoAsksForTheRightsAgain(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/a.jpg", "a", old)
	copied := e.run(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups"})
	e.write("/mnt/user/fotos/b.jpg", "b", old)
	moved := e.run(fileops.KindMove, map[string]any{"sources": []string{"/mnt/user/fotos/b.jpg"}, "dest": "/mnt/user/backups"})
	e.rules.set("/mnt/user/backups", ccfs.ReadOnly)
	for _, j := range []jobs.Job{copied, moved} {
		var out struct {
			Job *jobs.Job `json:"job"`
		}
		if code := e.call("anna", "POST", "/undo", map[string]string{"id": j.Undo}, &out); code < 300 && out.Job != nil {
			e.wait(out.Job.ID)
		}
	}
	if e.gone("/mnt/user/backups/a.jpg") {
		t.Error("undo deleted from a place that is read only by now")
	}
	if e.gone("/mnt/user/backups/b.jpg") {
		t.Error("undo moved out of a place that is read only by now")
	}
}

func TestReservedNamesCannotBeReachedOrMade(t *testing.T) {
	e := newEnv(t)
	if _, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/up.bin", Size: 5, Rule: "replace"}); code != http.StatusCreated {
		t.Fatalf("upload: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/.up.bin.cc-part", "name": "weg"}, nil); code < 400 {
		t.Errorf("rename of a part file: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/touch", map[string]string{"path": "/mnt/user/fotos/.neu.cc-part"}, nil); code < 400 {
		t.Errorf("touch of a part file: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/mkdir", map[string]string{"path": "/mnt/user/fotos/.cc-trash"}, nil); code < 400 {
		t.Errorf("mkdir of a bin: %d", code)
	}
}

func TestAnotherUsersTrashCannotBeReachedByPath(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Now())
	var j jobs.Job
	if code := e.call("bert", "POST", "/jobs", map[string]any{"kind": trash.KindTrash, "params": map[string]any{"paths": []string{"/mnt/user/fotos/a.jpg"}}}, &j); code != http.StatusAccepted {
		t.Fatalf("trash: %d", code)
	}
	waitAs(e, "bert", j.ID)
	if code := e.startAnswer(fileops.KindDelete, map[string]any{"paths": []string{"/mnt/user/fotos/.cc-trash"}}); code < 400 {
		t.Errorf("delete of the bin: %d", code)
	}
	if code := e.startAnswer(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/.cc-trash"}, "dest": "/mnt/user/backups"}); code < 400 {
		t.Errorf("copy of the bin: %d", code)
	}
}

func TestAnUploadDoesNotWriteThroughAPartFileSwappedForALink(t *testing.T) {
	linksOnly(t)
	for name, link := range map[string]func(old, new string) error{"symlink": os.Symlink, "hard link": os.Link} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.write("/mnt/user/backups/keep.txt", "keep", time.Now())
			up, code := e.startUpload("anna", fileops.UploadRequest{Path: "/mnt/user/fotos/up.bin", Size: 5, Rule: "replace"})
			if code != http.StatusCreated {
				t.Fatalf("upload: %d", code)
			}
			part := e.real("/mnt/user/fotos/.up.bin.cc-part")
			if err := os.Remove(part); err != nil {
				t.Fatal(err)
			}
			if err := link(e.real("/mnt/user/backups/keep.txt"), part); err != nil {
				t.Fatal(err)
			}
			e.chunk("anna", up.ID, 0, "HACK!")
			if got := e.read("/mnt/user/backups/keep.txt"); got != "keep" {
				t.Errorf("the chunk landed in the link's target: %q", got)
			}
		})
	}
}

func TestACopyDoesNotWriteThroughALinkPlantedAsItsPartFile(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Now())
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(outside, e.real("/mnt/user/backups/.a.jpg.cc-part")); err != nil {
		t.Fatal(err)
	}
	e.wait(e.start(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/backups"}).ID)
	if _, err := os.Lstat(outside); err == nil {
		t.Error("the copy wrote outside the source")
	}
}

func TestOperationsAnswerForWhereALinkLeads(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	e.write("/mnt/user/fotos/geheim/x.jpg", "x", time.Now())
	e.rules.set("/mnt/user/fotos/geheim", ccfs.Hidden)
	if err := os.Symlink(e.real("/mnt/user/fotos/geheim"), e.real("/mnt/user/backups/tor")); err != nil {
		t.Fatal(err)
	}
	if code := e.startAnswer(fileops.KindCopy, map[string]any{"sources": []string{"/mnt/user/backups/tor/x.jpg"}, "dest": "/mnt/user/backups"}); code != http.StatusForbidden {
		t.Errorf("copy through the link: %d", code)
	}
	if code := e.startAnswer(fileops.KindDelete, map[string]any{"paths": []string{"/mnt/user/backups/tor/x.jpg"}}); code != http.StatusForbidden {
		t.Errorf("delete through the link: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/touch", map[string]string{"path": "/mnt/user/backups/tor/neu.jpg"}, nil); code != http.StatusForbidden {
		t.Errorf("touch through the link: %d", code)
	}
}

func TestANewFolderWithItsEntriesIsTakenBackInOneStep(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Now())
	box := "/mnt/user/fotos/Mappe"
	j := e.run(fileops.KindMove, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": box, "rule": jobs.Skip, "newFolder": true})
	if e.read(box+"/a.jpg") != "a" {
		t.Fatal("the entry did not move")
	}
	var steps struct {
		Steps []undo.Step `json:"steps"`
	}
	e.call("anna", "GET", "/undo", nil, &steps)
	if len(steps.Steps) != 1 {
		t.Fatalf("%d steps", len(steps.Steps))
	}
	e.undo(j.Undo)
	if e.gone("/mnt/user/fotos/a.jpg") {
		t.Error("the entry did not come back")
	}
	if !e.gone(box) {
		t.Error("the folder stayed")
	}
}

func TestTakingBackANewFolderKeepsItWhenSomethingElseMovedIn(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Now())
	box := "/mnt/user/fotos/Mappe"
	j := e.run(fileops.KindMove, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": box, "rule": jobs.Skip, "newFolder": true})
	e.write(box+"/b.jpg", "b", time.Now())
	e.undo(j.Undo)
	if e.gone(box + "/b.jpg") {
		t.Error("the other file went away")
	}
}

func TestANewFolderHasToBeNew(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", time.Now())
	e.write("/mnt/user/fotos/Mappe/b.jpg", "b", time.Now())
	code := e.startAnswer(fileops.KindMove, map[string]any{"sources": []string{"/mnt/user/fotos/a.jpg"}, "dest": "/mnt/user/fotos/Mappe", "newFolder": true})
	if code != http.StatusConflict {
		t.Errorf("%d", code)
	}
}

func TestPreflightDoesNotDescribeHiddenEntries(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/geheim/x.jpg", "x", time.Now())
	e.rules.set("/mnt/user/fotos/geheim", ccfs.Hidden)
	var out map[string]any
	code := e.call("anna", "POST", "/ops/preflight", map[string]any{
		"kind": "copy", "sources": []string{"/mnt/user/backups"}, "dest": "/mnt/user/fotos", "names": []string{"geheim"},
	}, &out)
	raw, _ := json.Marshal(out)
	if code == http.StatusOK && strings.Contains(string(raw), `"entries"`) {
		t.Errorf("%s", raw)
	}
}
