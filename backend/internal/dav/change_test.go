package dav_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
)

func TestRangeRequestsWork(t *testing.T) {
	e := newEnv(t)
	a := e.ask("anna", "GET", "/mnt/user/fotos/a.jpg", "", hdr{"Range": "bytes=1-3"})
	e.want(a, http.StatusPartialContent)
	if a.body != "onn" {
		t.Fatalf("range body %q", a.body)
	}
}

func TestPutCreatesTheFileAndLeavesNoPart(t *testing.T) {
	e := newEnv(t)
	a := e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "wolke", nil)
	e.want(a, http.StatusCreated)
	if a.header.Get("ETag") == "" {
		t.Fatal("no ETag")
	}
	if e.read("fotos/b.jpg") != "wolke" {
		t.Fatal("wrong content")
	}
	entries, _ := os.ReadDir(e.real("fotos"))
	if len(entries) != 2 {
		t.Fatalf("left behind: %v", entries)
	}
	if got := e.ask("anna", "GET", "/mnt/user/fotos/b.jpg", "", nil).header.Get("ETag"); got != a.header.Get("ETag") {
		t.Fatalf("ETag changed from %s to %s", a.header.Get("ETag"), got)
	}
}

func TestPutWithoutAFolderIsAConflict(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "PUT", "/mnt/user/fehlt/b.jpg", "x", nil), http.StatusConflict)
	e.want(e.ask("anna", "MKCOL", "/mnt/user/fehlt/ordner", "", nil), http.StatusConflict)
}

func TestPutOnAFolderIsRefused(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos", "x", nil), http.StatusMethodNotAllowed)
}

func TestPutOfAPartialRangeIsRefused(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/a.jpg", "x", hdr{"Content-Range": "bytes 0-0/9"}), http.StatusBadRequest)
	if e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("the file changed")
	}
}

func TestBrokenUploadLeavesNoHalfFile(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/mnt/user/fotos/neu.jpg", "/mnt/user/fotos/a.jpg"} {
		body := io.MultiReader(strings.NewReader("halb"), iotest.ErrReader(errors.New("connection lost")))
		req := httptest.NewRequest("PUT", "/dav"+p, body)
		req.SetBasicAuth("anna", "pw")
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Fatalf("%s: status %d", p, rec.Code)
		}
	}
	if e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("a broken upload damaged the file")
	}
	entries, _ := os.ReadDir(e.real("fotos"))
	if len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestAConnectionCutOffDuringAnUploadLeavesNoHalfFile(t *testing.T) {
	e := newEnv(t)
	conn, err := net.Dial("tcp", e.srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	req := "PUT /dav/mnt/user/fotos/neu.jpg HTTP/1.1\r\nHost: x\r\nAuthorization: Basic YW5uYTpwdw==\r\nContent-Length: 100000\r\n\r\nhalb"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	e.srv.Close()
	entries, _ := os.ReadDir(e.real("fotos"))
	if len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestPutTakesTheTimeTheClientSends(t *testing.T) {
	e := newEnv(t)
	a := e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "x", hdr{"X-OC-Mtime": "1500000000"})
	e.want(a, http.StatusCreated)
	if a.header.Get("X-OC-Mtime") != "accepted" {
		t.Fatalf("X-OC-Mtime %q", a.header.Get("X-OC-Mtime"))
	}
	info, err := os.Stat(e.real("fotos/b.jpg"))
	if err != nil || info.ModTime().Unix() != 1500000000 {
		t.Fatalf("mtime %v, %v", info.ModTime(), err)
	}
	before := time.Now().Add(-time.Minute)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/c.jpg", "x", hdr{"X-OC-Mtime": "morgen"}), http.StatusCreated)
	if info, err := os.Stat(e.real("fotos/c.jpg")); err != nil || info.ModTime().Before(before) {
		t.Fatalf("mtime %v, %v", info.ModTime(), err)
	}
}

func TestCopyKeepsTheModificationTime(t *testing.T) {
	e := newEnv(t)
	old := time.Unix(1400000000, 0)
	if err := os.Chtimes(e.real("fotos/a.jpg"), old, old); err != nil {
		t.Fatal(err)
	}
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/b.jpg")), http.StatusCreated)
	if info, err := os.Stat(e.real("b.jpg")); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("mtime %v, %v", info.ModTime(), err)
	}
}

func TestAFolderCannotGoIntoItselfOrOverItsOwnParent(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/2026/b.jpg", "b")
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos", "", e.over("/mnt/user/fotos/2026/kopie")), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos", "", e.over("/mnt/user/fotos/2026")), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/2026", "", e.over("/mnt/user/fotos")), http.StatusForbidden)
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos/2026", "", e.over("/mnt/user/fotos")), http.StatusForbidden)
	if e.read("fotos/2026/b.jpg") != "b" || e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("files were lost")
	}
}

func TestOverwriteWithoutTheRightsLeavesTheTargetAlone(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/b.jpg", "b")
	e.want(e.ask("loescher", "COPY", "/mnt/user/fotos/a.jpg", "", e.over("/mnt/user/fotos/b.jpg")), http.StatusForbidden)
	e.want(e.ask("neuling", "COPY", "/mnt/user/fotos/a.jpg", "", e.over("/mnt/user/fotos/b.jpg")), http.StatusForbidden)
	if e.read("fotos/b.jpg") != "b" {
		t.Fatal("the target is gone")
	}
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos/a.jpg", "", e.over("/mnt/user/fotos/b.jpg")), http.StatusNoContent)
	if e.read("fotos/b.jpg") != "sonne" {
		t.Fatal("the overwrite did not happen")
	}
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/b.jpg")), http.StatusPreconditionFailed)
}

func TestLocksGuardWrites(t *testing.T) {
	e := newEnv(t)
	body := `<?xml version="1.0"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>anna</D:owner></D:lockinfo>`
	a := e.ask("anna", "LOCK", "/mnt/user/fotos/a.jpg", body, hdr{"Timeout": "Infinite", "Depth": "0"})
	e.want(a, http.StatusOK)
	token := a.header.Get("Lock-Token")
	if token == "" {
		t.Fatal("no lock token")
	}
	if !strings.Contains(a.body, "Second-") || strings.Contains(a.body, "Infinite") {
		t.Fatalf("a client that goes away must not keep a lock forever: %s", a.body)
	}
	e.want(e.ask("bea", "PUT", "/mnt/user/fotos/a.jpg", "x", nil), http.StatusLocked)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/a.jpg", "regen", hdr{"If": "(" + token + ")"}), http.StatusCreated)
	e.want(e.ask("anna", "UNLOCK", "/mnt/user/fotos/a.jpg", "", hdr{"Lock-Token": token}), http.StatusNoContent)
	e.want(e.ask("bea", "PUT", "/mnt/user/fotos/a.jpg", "x", nil), http.StatusCreated)
}

func TestLockCreatesAMissingFile(t *testing.T) {
	e := newEnv(t)
	body := `<?xml version="1.0"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`
	e.want(e.ask("anna", "LOCK", "/mnt/user/fotos/leer.txt", body, hdr{"Depth": "0"}), http.StatusCreated)
	if e.read("fotos/leer.txt") != "" {
		t.Fatal("the locked file should be empty")
	}
}

func TestProppatchAnswersWithoutChangingAnything(t *testing.T) {
	e := newEnv(t)
	body := `<?xml version="1.0"?><D:propertyupdate xmlns:D="DAV:" xmlns:Z="urn:schemas-microsoft-com:"><D:set><D:prop><Z:Win32LastAccessTime>x</Z:Win32LastAccessTime></D:prop></D:set></D:propertyupdate>`
	e.want(e.ask("anna", "PROPPATCH", "/mnt/user/fotos/a.jpg", body, nil), http.StatusMultiStatus)
	if e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("the file changed")
	}
}

func TestLabelsFollowTheFiles(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/b.jpg", "b")
	e.label("/mnt/user/fotos/a.jpg", "red")
	e.label("/mnt/user/fotos/b.jpg", "blue")

	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/c.jpg")), http.StatusCreated)
	if e.colour("/mnt/user/fotos/c.jpg") != "red" || e.colour("/mnt/user/fotos/a.jpg") != "" {
		t.Fatal("the label did not move")
	}
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos/c.jpg", "", e.dest("/mnt/user/d.jpg")), http.StatusCreated)
	if e.colour("/mnt/user/d.jpg") != "red" || e.colour("/mnt/user/fotos/c.jpg") != "red" {
		t.Fatal("the copy has no label")
	}
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/b.jpg", "", nil), http.StatusNoContent)
	if e.colour("/mnt/user/fotos/b.jpg") != "" {
		t.Fatal("the label outlived the file")
	}
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/c.jpg", "neu", nil), http.StatusCreated)
	if e.colour("/mnt/user/fotos/c.jpg") != "red" {
		t.Fatal("changing a file must keep its label")
	}
}

func TestChangesReachTheOtherPackages(t *testing.T) {
	e := newEnv(t)
	var seen []string
	stop := fschange.Observe(fschange.Observer{
		Created: func(p string) { seen = append(seen, "created "+p) },
		Moved:   func(from, to string) { seen = append(seen, "moved "+from+" "+to) },
		Removed: func(p string) { seen = append(seen, "removed "+p) },
	})
	defer stop()
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "x", nil), http.StatusCreated)
	e.want(e.ask("anna", "MKCOL", "/mnt/user/alben", "", nil), http.StatusCreated)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/b.jpg", "", e.dest("/mnt/user/alben/b.jpg")), http.StatusCreated)
	e.want(e.ask("anna", "DELETE", "/mnt/user/alben", "", nil), http.StatusNoContent)
	eq(t, seen, "created /mnt/user/fotos/b.jpg", "created /mnt/user/alben", "moved /mnt/user/fotos/b.jpg /mnt/user/alben/b.jpg", "removed /mnt/user/alben")
}

func TestChangesAreAnnouncedOnTheFolderTopic(t *testing.T) {
	e := newEnv(t)
	c, _, _ := e.deps.Hub.Connect("x", "anna", 0)
	defer e.deps.Hub.Disconnect(c)
	e.deps.Hub.Subscribe("x", "anna", hub.FolderTopic("/mnt/user/fotos"))

	next := func() watch.Change {
		t.Helper()
		select {
		case ev := <-c.Events:
			if ev.Type != watch.EventChanged {
				t.Fatalf("event %s", ev.Type)
			}
			var ch watch.Change
			if err := json.Unmarshal(ev.Data, &ch); err != nil {
				t.Fatal(err)
			}
			return ch
		case <-time.After(5 * time.Second):
			t.Fatal("no event")
		}
		return watch.Change{}
	}

	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "x", nil), http.StatusCreated)
	if ch := next(); ch.Path != "/mnt/user/fotos" || !slices.Contains(ch.Names, "b.jpg") {
		t.Fatalf("put: %+v", ch)
	}
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/b.jpg", "", e.dest("/mnt/user/fotos/c.jpg")), http.StatusCreated)
	if ch := next(); len(ch.Renamed) != 1 || ch.Renamed[0].From != "b.jpg" || ch.Renamed[0].To != "c.jpg" {
		t.Fatalf("rename: %+v", ch)
	}
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/c.jpg", "", nil), http.StatusNoContent)
	if ch := next(); !slices.Contains(ch.Removed, "c.jpg") {
		t.Fatalf("delete: %+v", ch)
	}
}

func TestTheIndexIsToldAboutEveryFolderThatChanged(t *testing.T) {
	e := newEnv(t)
	e.write("alben/x", "x")
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/alben/a.jpg")), http.StatusCreated)
	e.index.mu.Lock()
	defer e.index.mu.Unlock()
	for _, dir := range []string{"/fotos", "/alben"} {
		if !slices.Contains(e.index.dirs, dir) {
			t.Fatalf("%s was not refreshed: %v", dir, e.index.dirs)
		}
	}
}

func TestReadingAnnouncesNothing(t *testing.T) {
	e := newEnv(t)
	e.ask("anna", "GET", "/mnt/user/fotos/a.jpg", "", nil)
	e.ask("anna", "PROPFIND", "/mnt/user/fotos", "", hdr{"Depth": "1"})
	e.index.mu.Lock()
	defer e.index.mu.Unlock()
	if len(e.index.dirs) != 0 {
		t.Fatalf("refreshed %v", e.index.dirs)
	}
}
