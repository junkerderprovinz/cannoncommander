package dav_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

func TestSwitchedOffAnswersNotFoundToEveryMethod(t *testing.T) {
	e := newEnv(t)
	e.on = false
	for _, m := range []string{"OPTIONS", "PROPFIND", "GET", "PUT", "DELETE", "LOCK"} {
		e.want(e.ask("admin", m, "/mnt/user/fotos/a.jpg", "", nil), http.StatusNotFound)
	}
	if e.signIns != 0 {
		t.Fatal("a server that is off has no business checking credentials")
	}
	e.on = true
	e.want(e.ask("admin", "GET", "/mnt/user/fotos/a.jpg", "", nil), http.StatusOK)
}

func TestSignInIsRequiredAndChallenged(t *testing.T) {
	e := newEnv(t)
	for _, a := range []answer{e.ask("", "PROPFIND", "/", "", hdr{"Depth": "0"}), e.askAs("anna", "wrong", "GET", "/mnt/user/fotos/a.jpg", "", nil)} {
		e.want(a, http.StatusUnauthorized)
		if !strings.HasPrefix(a.header.Get("WWW-Authenticate"), "Basic ") {
			t.Fatalf("challenge %q", a.header.Get("WWW-Authenticate"))
		}
	}
}

func TestLockedOutClientIsToldHowLongToWait(t *testing.T) {
	e := newEnv(t)
	a := e.askAs("anna", "locked", "GET", "/mnt/user/fotos/a.jpg", "", nil)
	e.want(a, http.StatusTooManyRequests)
	if a.header.Get("Retry-After") != "91" {
		t.Fatalf("Retry-After %q", a.header.Get("Retry-After"))
	}
}

func TestBrowserRequestsFromPagesAreRefused(t *testing.T) {
	e := newEnv(t)
	for _, site := range []string{"cross-site", "same-site", "same-origin"} {
		e.want(e.ask("anna", "GET", "/mnt/user/fotos/a.jpg", "", hdr{"Sec-Fetch-Site": site}), http.StatusForbidden)
	}
	e.want(e.ask("anna", "GET", "/mnt/user/fotos/a.jpg", "", hdr{"Sec-Fetch-Site": "none"}), http.StatusOK)
}

func TestAFileCannotRunInTheOriginOfTheApp(t *testing.T) {
	e := newEnv(t)
	e.write("seite.html", "<script>alert(1)</script>")
	a := e.ask("anna", "GET", "/mnt/user/seite.html", "", nil)
	e.want(a, http.StatusOK)
	if !strings.Contains(a.header.Get("Content-Security-Policy"), "sandbox") || a.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", a.header)
	}
	if !strings.HasPrefix(a.header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("disposition %q", a.header.Get("Content-Disposition"))
	}
}

func TestPostIsNotPartOfTheServer(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "POST", "/mnt/user/fotos/a.jpg", "", nil), http.StatusMethodNotAllowed)
}

func TestRootListsTheSourcesLikeTheAppsPaths(t *testing.T) {
	e := newEnv(t)
	eq(t, e.list("anna", "/"), "mnt")
	eq(t, e.list("anna", "/mnt"), "usb", "user")
	eq(t, e.list("anna", "/mnt/user"), "fotos")
	eq(t, e.list("anna", "/mnt/user/fotos"), "a.jpg")
}

func TestDisksAndPoolsAreForAdminsOnly(t *testing.T) {
	e := newEnv(t)
	eq(t, e.list("admin", "/mnt"), "disk1", "usb", "user")
	if err := os.WriteFile(filepath.Join(e.disk, "x"), []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	e.want(e.ask("admin", "GET", "/mnt/disk1/x", "", nil), http.StatusOK)
	e.want(e.ask("anna", "PROPFIND", "/mnt/disk1", "", hdr{"Depth": "0"}), http.StatusForbidden)
	e.want(e.ask("anna", "GET", "/mnt/disk1/x", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "PUT", "/mnt/disk1/y", "y", nil), http.StatusForbidden)
}

func TestScopedUserSeesOnlyThePathToTheScope(t *testing.T) {
	e := newEnv(t)
	e.write("privat/b.txt", "b")
	eq(t, e.list("bea", "/mnt/user"), "fotos")
	e.want(e.ask("bea", "GET", "/mnt/user/fotos/a.jpg", "", nil), http.StatusOK)
	e.want(e.ask("bea", "GET", "/mnt/user/privat/b.txt", "", nil), http.StatusForbidden)
	e.want(e.ask("bea", "PUT", "/mnt/user/neu.txt", "n", nil), http.StatusForbidden)
}

func TestBookkeepingStaysInvisibleAndUnreachable(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/.cc-trash/alt.jpg", "alt")
	e.write("fotos/.a.jpg.cc-part", "halb")
	eq(t, e.list("admin", "/mnt/user/fotos"), "a.jpg")
	for _, p := range []string{"/mnt/user/fotos/.cc-trash", "/mnt/user/fotos/.cc-trash/alt.jpg", "/mnt/user/fotos/.a.jpg.cc-part"} {
		e.want(e.ask("admin", "PROPFIND", p, "", hdr{"Depth": "0"}), http.StatusNotFound)
	}
	e.want(e.ask("admin", "GET", "/mnt/user/fotos/.cc-trash/alt.jpg", "", nil), http.StatusNotFound)
	e.want(e.ask("admin", "PUT", "/mnt/user/fotos/.cc-trash/neu.jpg", "n", nil), http.StatusForbidden)
	e.want(e.ask("admin", "PUT", "/mnt/user/fotos/.b.jpg.cc-part", "n", nil), http.StatusForbidden)
	e.want(e.ask("admin", "DELETE", "/mnt/user/fotos/.cc-trash", "", nil), http.StatusForbidden)
	e.want(e.ask("admin", "MKCOL", "/mnt/user/.cc-trash", "", nil), http.StatusForbidden)
	e.want(e.ask("admin", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/.cc-trash/a.jpg")), http.StatusForbidden)
	if !e.exists("fotos/.cc-trash/alt.jpg") || e.exists("fotos/.cc-trash/a.jpg") {
		t.Fatal("the trash was touched")
	}
}

func TestOrdinaryDotFilesAreShown(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/.versteckt", "x")
	eq(t, e.list("anna", "/mnt/user/fotos"), ".versteckt", "a.jpg")
	e.want(e.ask("anna", "MKCOL", "/mnt/user/fotos/.ordner", "", nil), http.StatusCreated)
}

func TestArchivePathsAreNotExposed(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/album.zip", "zip")
	eq(t, e.list("anna", "/mnt/user/fotos"), "a.jpg", "album.zip")
	e.want(e.ask("anna", "PROPFIND", "/mnt/user/fotos/album.zip::/innen", "", hdr{"Depth": "0"}), http.StatusNotFound)
	e.want(e.ask("anna", "GET", "/mnt/user/fotos/album.zip::/innen", "", nil), http.StatusNotFound)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/album.zip::/innen", "x", nil), http.StatusForbidden)
}

func TestHiddenRulePathIsOutOfReach(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/geheim"] = ccfs.Hidden
	e.write("geheim/plan.txt", "plan")
	eq(t, e.list("anna", "/mnt/user"), "fotos")
	eq(t, e.list("admin", "/mnt/user"), "fotos", "geheim")
	e.want(e.ask("anna", "PROPFIND", "/mnt/user/geheim", "", hdr{"Depth": "0"}), http.StatusForbidden)
	e.want(e.ask("anna", "GET", "/mnt/user/geheim/plan.txt", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "PUT", "/mnt/user/geheim/neu.txt", "n", nil), http.StatusForbidden)
	e.want(e.ask("anna", "DELETE", "/mnt/user/geheim/plan.txt", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/geheim/plan.txt", "", e.dest("/mnt/user/fotos/plan.txt")), http.StatusForbidden)
	e.want(e.ask("anna", "COPY", "/mnt/user/geheim/plan.txt", "", e.dest("/mnt/user/fotos/plan.txt")), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/geheim/a.jpg")), http.StatusForbidden)
	if !e.exists("geheim/plan.txt") || e.exists("fotos/plan.txt") || e.exists("geheim/a.jpg") {
		t.Fatal("something moved past the rule")
	}
}

func TestAFolderHoldingSomethingHiddenCannotBeRemovedOrMoved(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/fotos/geheim"] = ccfs.Hidden
	e.write("fotos/geheim/plan.txt", "plan")
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos", "", e.dest("/mnt/user/alben")), http.StatusForbidden)
	e.write("ziel/x", "x")
	e.want(e.ask("anna", "MOVE", "/mnt/user/ziel", "", e.over("/mnt/user/fotos")), http.StatusForbidden)
	if !e.exists("fotos/geheim/plan.txt") || !e.exists("ziel/x") {
		t.Fatal("something is gone")
	}
}

func TestAFolderHoldingSomethingReadOnlyCannotBeRemoved(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/fotos/fest"] = ccfs.ReadOnly
	e.write("fotos/fest/plan.txt", "plan")
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos", "", nil), http.StatusForbidden)
	if !e.exists("fotos/fest/plan.txt") {
		t.Fatal("the read only file is gone")
	}
}

func TestCopyOfAFolderLeavesHiddenEntriesBehind(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/fotos/geheim"] = ccfs.Hidden
	e.write("fotos/geheim/plan.txt", "plan")
	e.want(e.ask("anna", "COPY", "/mnt/user/fotos", "", e.dest("/mnt/user/kopie")), http.StatusCreated)
	if e.read("kopie/a.jpg") != "sonne" || e.exists("kopie/geheim") {
		t.Fatal("the copy does not match what anna sees")
	}
}

func TestReadOnlyRuleLetsReadsThrough(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/fotos"] = ccfs.ReadOnly
	e.want(e.ask("anna", "GET", "/mnt/user/fotos/a.jpg", "", nil), http.StatusOK)
	eq(t, e.list("anna", "/mnt/user/fotos"), "a.jpg")
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/a.jpg", "x", nil), http.StatusForbidden)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "x", nil), http.StatusForbidden)
	e.want(e.ask("anna", "DELETE", "/mnt/user/fotos/a.jpg", "", nil), http.StatusForbidden)
	if e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("the file changed")
	}
}

func TestReadOnlySourceRefusesEveryWrite(t *testing.T) {
	e := newEnv(t)
	if err := os.WriteFile(filepath.Join(e.usb, "liesmich.txt"), []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method, p string
		headers   hdr
	}{
		{"MKCOL", "/mnt/usb/ordner", nil},
		{"PUT", "/mnt/usb/neu.txt", nil},
		{"PUT", "/mnt/usb/liesmich.txt", nil},
		{"DELETE", "/mnt/usb/liesmich.txt", nil},
		{"COPY", "/mnt/usb/liesmich.txt", e.dest("/mnt/usb/kopie.txt")},
		{"MOVE", "/mnt/usb/liesmich.txt", e.dest("/mnt/usb/weg.txt")},
		{"MOVE", "/mnt/usb/liesmich.txt", e.dest("/mnt/user/weg.txt")},
		{"COPY", "/mnt/user/fotos/a.jpg", e.dest("/mnt/usb/a.jpg")},
		{"PROPPATCH", "/mnt/usb/liesmich.txt", nil},
		{"LOCK", "/mnt/usb/liesmich.txt", nil},
	}
	for _, c := range cases {
		a := e.ask("anna", c.method, c.p, "", c.headers)
		if a.status != http.StatusForbidden {
			t.Errorf("%s %s: status %d", c.method, c.p, a.status)
		}
	}
	entries, err := os.ReadDir(e.usb)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the source changed: %v %v", entries, err)
	}
	eq(t, e.list("anna", "/mnt/usb"), "liesmich.txt")
	e.want(e.ask("anna", "GET", "/mnt/usb/liesmich.txt", "", nil), http.StatusOK)
	e.want(e.ask("anna", "COPY", "/mnt/usb/liesmich.txt", "", e.dest("/mnt/user/kopie.txt")), http.StatusCreated)
}

func TestNewPutTargetNeedsTheCreateRight(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("bearbeiter", "PUT", "/mnt/user/fotos/neu.jpg", "x", nil), http.StatusForbidden)
	if e.exists("fotos/neu.jpg") {
		t.Fatal("the new file exists")
	}
	e.want(e.ask("bearbeiter", "PUT", "/mnt/user/fotos/a.jpg", "regen", nil), http.StatusCreated)
	if e.read("fotos/a.jpg") != "regen" {
		t.Fatal("the existing file did not change")
	}
}

func TestReplacingAFileNeedsTheModifyRight(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("neuling", "PUT", "/mnt/user/fotos/a.jpg", "regen", nil), http.StatusForbidden)
	if e.read("fotos/a.jpg") != "sonne" {
		t.Fatal("the file changed")
	}
	e.want(e.ask("neuling", "PUT", "/mnt/user/fotos/neu.jpg", "x", nil), http.StatusCreated)
}

func TestEachMethodNeedsItsRight(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("gast", "MKCOL", "/mnt/user/ordner", "", nil), http.StatusForbidden)
	e.want(e.ask("neuling", "MKCOL", "/mnt/user/ordner", "", nil), http.StatusCreated)
	e.want(e.ask("neuling", "DELETE", "/mnt/user/fotos/a.jpg", "", nil), http.StatusForbidden)
	e.want(e.ask("gast", "PROPPATCH", "/mnt/user/fotos/a.jpg", "", nil), http.StatusForbidden)
	e.want(e.ask("neuling", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/b.jpg")), http.StatusForbidden)
	e.want(e.ask("neuling", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/a.jpg")), http.StatusForbidden)
	e.want(e.ask("gast", "COPY", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/b.jpg")), http.StatusForbidden)
	e.want(e.ask("loescher", "DELETE", "/mnt/user/fotos/a.jpg", "", nil), http.StatusNoContent)
}

func TestListingWorksWithoutTheDownloadRight(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/ohne-endung", "geheimer inhalt")
	a := e.ask("seher", "PROPFIND", "/mnt/user/fotos", "", hdr{"Depth": "1"})
	e.want(a, http.StatusMultiStatus)
	if !strings.Contains(a.body, "ohne-endung") || !strings.Contains(a.body, "getcontentlength") {
		t.Fatalf("listing: %s", a.body)
	}
	e.want(e.ask("seher", "GET", "/mnt/user/fotos/a.jpg", "", nil), http.StatusForbidden)
	e.want(e.ask("seher", "HEAD", "/mnt/user/fotos/a.jpg", "", nil), http.StatusForbidden)
	e.want(e.ask("seher", "COPY", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/b.jpg")), http.StatusForbidden)
}

func TestReadOnlyTokenCannotWrite(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct {
		method  string
		headers hdr
	}{
		{"PUT", nil}, {"DELETE", nil}, {"MKCOL", nil}, {"PROPPATCH", nil}, {"LOCK", nil},
		{"UNLOCK", hdr{"Lock-Token": "<1>"}},
		{"MOVE", e.dest("/mnt/user/fotos/b.jpg")}, {"COPY", e.dest("/mnt/user/fotos/b.jpg")},
	} {
		a := e.askAs("anna", "r", c.method, "/mnt/user/fotos/a.jpg", "", c.headers)
		if a.status != http.StatusForbidden {
			t.Errorf("%s: status %d", c.method, a.status)
		}
	}
	if e.read("fotos/a.jpg") != "sonne" || e.exists("fotos/b.jpg") {
		t.Fatal("the token changed something")
	}
	e.want(e.askAs("anna", "r", "GET", "/mnt/user/fotos/a.jpg", "", nil), http.StatusOK)
	e.want(e.askAs("anna", "r", "PROPFIND", "/mnt/user/fotos", "", hdr{"Depth": "1"}), http.StatusMultiStatus)
	e.want(e.askAs("anna", "r", "OPTIONS", "/mnt/user/fotos", "", nil), http.StatusOK)
}

func TestOptionsNamesWhatTheUserMayDo(t *testing.T) {
	e := newEnv(t)
	allow := func(a answer) []string {
		t.Helper()
		e.want(a, http.StatusOK)
		return strings.Split(a.header.Get("Allow"), ", ")
	}
	file := allow(e.ask("anna", "OPTIONS", "/mnt/user/fotos/a.jpg", "", nil))
	for _, m := range []string{"GET", "PUT", "DELETE", "MOVE", "COPY", "PROPFIND", "LOCK"} {
		if !slices.Contains(file, m) {
			t.Errorf("anna's file misses %s: %v", m, file)
		}
	}
	if a := e.ask("anna", "OPTIONS", "/mnt/user/fotos", "", nil); !strings.Contains(a.header.Get("DAV"), "2") {
		t.Fatalf("DAV %q: Windows and macOS only write where LOCK is announced", a.header.Get("DAV"))
	}
	gast := allow(e.ask("gast", "OPTIONS", "/mnt/user/fotos/a.jpg", "", nil))
	for _, m := range []string{"PUT", "DELETE", "MOVE", "LOCK", "PROPPATCH"} {
		if slices.Contains(gast, m) {
			t.Errorf("a reader is offered %s: %v", m, gast)
		}
	}
	token := allow(e.askAs("anna", "r", "OPTIONS", "/mnt/user/fotos/a.jpg", "", nil))
	if slices.Contains(token, "PUT") {
		t.Errorf("a read only token is offered PUT: %v", token)
	}
	missing := allow(e.ask("anna", "OPTIONS", "/mnt/user/fotos/neu", "", nil))
	if !slices.Contains(missing, "PUT") || !slices.Contains(missing, "MKCOL") || slices.Contains(missing, "GET") {
		t.Errorf("a free name: %v", missing)
	}
	if slices.Contains(allow(e.ask("seher", "OPTIONS", "/mnt/user/fotos/a.jpg", "", nil)), "GET") {
		t.Error("GET is offered without the download right")
	}
}

func TestEndlessDepthIsRefused(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "PROPFIND", "/mnt/user", "", hdr{"Depth": "infinity"}), http.StatusForbidden)
	e.want(e.ask("anna", "PROPFIND", "/mnt/user", "", nil), http.StatusForbidden)
}

func TestLinkLeavingTheSourceStaysClosed(t *testing.T) {
	e := newEnv(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "passwd"), []byte("root"), 0o666); err != nil {
		t.Fatal(err)
	}
	e.symlink(outside, "fotos/raus")
	e.symlink(filepath.Join(outside, "passwd"), "fotos/raus.txt")
	e.symlink(filepath.Join(outside, "gibtsnicht"), "fotos/tot")
	eq(t, e.list("admin", "/mnt/user/fotos"), "a.jpg")
	e.want(e.ask("admin", "GET", "/mnt/user/fotos/raus.txt", "", nil), http.StatusForbidden)
	e.want(e.ask("admin", "GET", "/mnt/user/fotos/raus/passwd", "", nil), http.StatusForbidden)
	e.want(e.ask("admin", "PROPFIND", "/mnt/user/fotos/raus", "", hdr{"Depth": "0"}), http.StatusForbidden)
	e.want(e.ask("admin", "PUT", "/mnt/user/fotos/raus/neu", "x", nil), http.StatusForbidden)
	e.want(e.ask("admin", "PUT", "/mnt/user/fotos/raus.txt", "x", nil), http.StatusForbidden)
	e.want(e.ask("admin", "PUT", "/mnt/user/fotos/tot", "x", nil), http.StatusForbidden)
	e.want(e.ask("admin", "MKCOL", "/mnt/user/fotos/raus/neu", "", nil), http.StatusForbidden)
	e.want(e.ask("admin", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/fotos/raus/a.jpg")), http.StatusForbidden)
	if _, err := os.Stat(filepath.Join(outside, "neu")); err == nil {
		t.Fatal("a file landed outside the source")
	}
	if b, _ := os.ReadFile(filepath.Join(outside, "passwd")); string(b) != "root" {
		t.Fatal("a file outside the source changed")
	}
}

func TestLinkIntoAPlaceTheUserMayNotSeeStaysClosed(t *testing.T) {
	e := newEnv(t)
	e.rules["/mnt/user/geheim"] = ccfs.Hidden
	e.write("geheim/plan.txt", "plan")
	e.write("privat/b.txt", "b")
	e.write("fotos/.cc-trash/alt.jpg", "alt")
	e.symlink(e.real("geheim"), "fotos/zugang")
	e.symlink(e.real("privat"), "fotos/nachbar")
	e.symlink(e.real("fotos/.cc-trash"), "fotos/muell")
	eq(t, e.list("anna", "/mnt/user/fotos"), "a.jpg", "nachbar")
	e.want(e.ask("anna", "GET", "/mnt/user/fotos/zugang/plan.txt", "", nil), http.StatusForbidden)
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/zugang/neu.txt", "x", nil), http.StatusForbidden)
	e.want(e.ask("admin", "GET", "/mnt/user/fotos/muell/alt.jpg", "", nil), http.StatusNotFound)
	e.want(e.ask("bea", "GET", "/mnt/user/fotos/nachbar/b.txt", "", nil), http.StatusForbidden)
	eq(t, e.list("bea", "/mnt/user/fotos"), "a.jpg")
}

func TestLinkInsideTheSourceWorks(t *testing.T) {
	e := newEnv(t)
	e.write("alben/b.jpg", "wolke")
	e.symlink(e.real("alben"), "fotos/kurz")
	e.symlink(e.real("alben/b.jpg"), "fotos/b.jpg")
	if a := e.ask("anna", "GET", "/mnt/user/fotos/kurz/b.jpg", "", nil); a.body != "wolke" {
		t.Fatalf("read through the link: %q", a.body)
	}
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/b.jpg", "regen", nil), http.StatusCreated)
	if e.read("alben/b.jpg") != "regen" {
		t.Fatal("a write to a link has to reach its target")
	}
	if info, err := os.Lstat(e.real("fotos/b.jpg")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link is gone: %v %v", info, err)
	}
}

func TestSourceRootsAndVirtualFoldersStayPut(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/mnt/user", "/mnt", "/"} {
		e.want(e.ask("admin", "DELETE", p, "", nil), http.StatusForbidden)
		e.want(e.ask("admin", "MOVE", p, "", e.dest("/mnt/user/fotos/weg")), http.StatusForbidden)
	}
	e.want(e.ask("admin", "MKCOL", "/mnt/neu", "", nil), http.StatusNotFound)
	e.want(e.ask("admin", "PUT", "/neu.txt", "x", nil), http.StatusNotFound)
	if !e.exists("fotos/a.jpg") {
		t.Fatal("the source lost its files")
	}
}

func TestPathsWithDotsCannotClimbOut(t *testing.T) {
	e := newEnv(t)
	e.want(e.ask("anna", "MOVE", "/mnt/user/fotos/a.jpg", "", e.dest("/mnt/user/../../../x")), http.StatusBadRequest)
	if !e.exists("fotos/a.jpg") {
		t.Fatal("the file moved")
	}
}
