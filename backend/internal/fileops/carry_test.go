package fileops_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

func albumWithThumbs(e *env) string {
	old := time.Now().Add(-time.Hour)
	e.write("/mnt/user/fotos/album/bild.jpg", "bild", old)
	e.write("/mnt/user/fotos/album/Thumbs.db", "klein", old)
	e.write("/mnt/user/fotos/album/sub/Thumbs.db", "klein", old)
	e.rules.hideName("Thumbs.db")
	return "/mnt/user/fotos/album"
}

func TestAFolderTakesAFileHiddenByNameAlong(t *testing.T) {
	t.Run("delete", func(t *testing.T) {
		e := newEnv(t)
		album := albumWithThumbs(e)
		e.run(fileops.KindDelete, map[string]any{"paths": []string{album}})
		if !e.gone(album) {
			t.Error("the folder is still there")
		}
	})
	t.Run("trash and back", func(t *testing.T) {
		e := newEnv(t)
		album := albumWithThumbs(e)
		e.run(trash.KindTrash, map[string]any{"paths": []string{album}})
		if !e.gone(album) {
			t.Fatal("the folder is still there")
		}
		items := e.trashItems()
		if len(items) != 1 {
			t.Fatalf("%d items in the trash", len(items))
		}
		var restore jobs.Job
		if code := e.call("anna", "POST", "/trash/restore", map[string]any{"ids": []string{items[0].ID}}, &restore); code != http.StatusAccepted {
			t.Fatalf("restore: %d", code)
		}
		e.wait(restore.ID)
		if e.read(album+"/Thumbs.db") != "klein" {
			t.Error("the hidden file did not come back")
		}
	})
	t.Run("move", func(t *testing.T) {
		e := newEnv(t)
		album := albumWithThumbs(e)
		e.run(fileops.KindMove, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups"})
		if !e.gone(album) {
			t.Error("the folder is still there")
		}
		if e.read("/mnt/user/backups/album/sub/Thumbs.db") != "klein" {
			t.Error("the hidden file stayed behind")
		}
	})
	t.Run("rename", func(t *testing.T) {
		e := newEnv(t)
		album := albumWithThumbs(e)
		if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": album, "name": "anders"}, nil); code != http.StatusOK {
			t.Fatalf("rename: %d", code)
		}
		if e.read("/mnt/user/fotos/anders/Thumbs.db") != "klein" {
			t.Error("the hidden file did not keep its place")
		}
	})
}

func TestAMoveIntoAnExistingFolderTakesAFileHiddenByNameAlong(t *testing.T) {
	e := newEnv(t)
	album := albumWithThumbs(e)
	e.write("/mnt/user/backups/album/alt.jpg", "alt", time.Now())
	j := e.run(fileops.KindMove, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups", "rule": jobs.Replace})
	if !e.gone(album) {
		t.Error("the folder is still there")
	}
	if e.read("/mnt/user/backups/album/Thumbs.db") != "klein" {
		t.Error("the hidden file stayed behind")
	}
	e.undo(j.Undo)
	if e.read(album+"/Thumbs.db") != "klein" {
		t.Error("undo left the hidden file behind")
	}
}

func TestAMoveAsksNothingAboutAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	album := albumWithThumbs(e)
	e.write("/mnt/user/backups/album/Thumbs.db", "alt", time.Now())
	e.run(fileops.KindMove, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups", "rule": jobs.Replace})
	if e.read("/mnt/user/backups/album/Thumbs.db") != "alt" {
		t.Error("the hidden file at the target was replaced")
	}
	if e.read(album+"/Thumbs.db") != "klein" {
		t.Error("the hidden file at the source went away")
	}
	if e.read("/mnt/user/backups/album/bild.jpg") != "bild" {
		t.Error("the open file did not move")
	}
}

func TestACopyLeavesOutAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	album := albumWithThumbs(e)
	e.run(fileops.KindCopy, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups"})
	if e.gone("/mnt/user/backups/album/bild.jpg") {
		t.Error("the open file is missing")
	}
	if !e.gone("/mnt/user/backups/album/Thumbs.db") {
		t.Error("the hidden file came along")
	}
}

func TestAFileHiddenByNameCannotBeTakenByItself(t *testing.T) {
	e := newEnv(t)
	albumWithThumbs(e)
	file := "/mnt/user/fotos/album/Thumbs.db"
	if code := e.startAnswer(fileops.KindDelete, map[string]any{"paths": []string{file}}); code != http.StatusForbidden {
		t.Errorf("delete: %d", code)
	}
	if code := e.startAnswer(fileops.KindMove, map[string]any{"sources": []string{file}, "dest": "/mnt/user/backups"}); code != http.StatusForbidden {
		t.Errorf("move: %d", code)
	}
}

func TestAPathRuleStillStopsAFolderThatAlsoHoldsAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	album := albumWithThumbs(e)
	e.write(album+"/privat/x.jpg", "x", time.Now())
	e.rules.set(album+"/privat", ccfs.Hidden)
	if code := e.startAnswer(fileops.KindDelete, map[string]any{"paths": []string{album}}); code != http.StatusForbidden {
		t.Errorf("delete: %d", code)
	}
	if code := e.startAnswer(fileops.KindMove, map[string]any{"sources": []string{album}, "dest": "/mnt/user/backups"}); code != http.StatusForbidden {
		t.Errorf("move: %d", code)
	}
}
