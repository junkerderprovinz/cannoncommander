package tools

import (
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

func TestAttrsLeaveOutAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/album/bild.txt", "bild")
	e.write("/mnt/user/fotos/album/Thumbs.db", "klein")
	e.rules.hideName("Thumbs.db")
	before, err := os.Stat(e.real("/mnt/user/fotos/album/Thumbs.db"))
	if err != nil {
		t.Fatal(err)
	}
	e.run(KindAttrs, AttrsParams{Paths: []string{"/mnt/user/fotos/album"}, Mode: "700", Recursive: true})
	if runtime.GOOS == "windows" {
		return
	}
	after, err := os.Stat(e.real("/mnt/user/fotos/album/Thumbs.db"))
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode() != before.Mode() {
		t.Errorf("the hidden file went from %v to %v", before.Mode(), after.Mode())
	}
	if open, _ := os.Stat(e.real("/mnt/user/fotos/album/bild.txt")); open.Mode().Perm() != 0o700 {
		t.Errorf("the open file has %v", open.Mode())
	}
}

func TestSyncDeletesAFolderWithAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/backups/album/bild.txt", "bild")
	e.write("/mnt/user/backups/album/Thumbs.db", "klein")
	e.write("/mnt/user/fotos/ruhe.txt", "ruhe")
	e.rules.hideName("Thumbs.db")
	e.run(KindSync, SyncParams{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Steps: []SyncStep{{Rel: "album", Act: "delete", To: "R"}}})
	if e.exists("/mnt/user/backups/album") {
		t.Error("the folder is still there")
	}
}

func TestArrowLoopGetsNoFolderWithAFileHiddenByName(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Thumbs.db", "klein")
	e.rules.hideName("Thumbs.db")
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("ArrowLoop was asked")
	}))
	defer server.Close()
	e.al = ArrowLoopConfig{URL: server.URL, Token: "t"}
	code := e.call("anna", "POST", "/tools/arrowloop", ArrowLoopRequest{Name: "x", Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Direction: "right", Every: "daily", At: "03:15"}, nil)
	if code != http.StatusForbidden {
		t.Errorf("%d", code)
	}
}

func TestAttrsStillStopAtAPathRuleBesideAHiddenName(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.write("/mnt/user/fotos/Thumbs.db", "klein")
	e.rules.hideName("Thumbs.db")
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos"}, Mode: "700", Recursive: true}, nil); code != http.StatusForbidden {
		t.Errorf("%d", code)
	}
}
