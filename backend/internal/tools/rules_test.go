package tools

import (
	"archive/tar"
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

func linksOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs symlinks")
	}
}

func protectedShare(e *env, effect ccfs.Effect) {
	e.write("/mnt/user/fotos/frei.txt", "frei")
	e.write("/mnt/user/fotos/geheim/x.txt", "geheim")
	e.rules.set("/mnt/user/fotos/geheim", effect)
}

// startAnswer posts a job and reports the status, without waiting for it.
func (e *env) startAnswer(kind string, params any) int {
	e.t.Helper()
	return e.call("anna", "POST", "/jobs", map[string]any{"kind": kind, "params": params}, nil)
}

func TestSyncDoesNotFollowAStepIntoAHiddenPlace(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	code := e.startAnswer(KindSync, SyncParams{Left: "/mnt/user/fotos", Right: "/mnt/user/backups",
		Steps: []SyncStep{{Rel: "geheim/x.txt", Act: "copy", To: "R"}}})
	if code != http.StatusForbidden {
		t.Errorf("sync step into a hidden folder: %d", code)
	}
	if e.exists("/mnt/user/backups/geheim/x.txt") {
		t.Error("the hidden file was copied")
	}
}

func TestSyncDoesNotWriteOverAReadOnlyPlace(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/backups/geheim/x.txt", "neu")
	protectedShare(e, ccfs.ReadOnly)
	for _, act := range []string{"replace", "delete"} {
		code := e.startAnswer(KindSync, SyncParams{Left: "/mnt/user/fotos", Right: "/mnt/user/backups",
			Steps: []SyncStep{{Rel: "geheim/x.txt", Act: act, To: "L"}}})
		if code != http.StatusForbidden {
			t.Errorf("%s: %d", act, code)
		}
	}
	if got := e.read("/mnt/user/fotos/geheim/x.txt"); got != "geheim" {
		t.Errorf("the read only file changed: %q", got)
	}
}

func TestSyncCopyLeavesOutWhatIsHiddenInside(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.write("/mnt/user/backups/.keep", "")
	e.run(KindSync, SyncParams{Left: "/mnt/user", Right: "/mnt/user/downloads",
		Steps: []SyncStep{{Rel: "fotos", Act: "copy", To: "R"}}})
	if !e.exists("/mnt/user/downloads/fotos/frei.txt") {
		t.Error("the open file is missing")
	}
	if e.exists("/mnt/user/downloads/fotos/geheim/x.txt") {
		t.Error("the hidden file came along")
	}
}

func TestCompareLeavesOutWhatIsHidden(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.write("/mnt/user/backups/geheim/x.txt", "x")
	var out struct {
		Items []Compared `json:"items"`
	}
	code := e.call("anna", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Recursive: true}, &out)
	if code != http.StatusOK {
		t.Fatalf("compare: %d", code)
	}
	for _, it := range out.Items {
		if it.Left != nil && (it.Name == "geheim" || it.Name == "x.txt") {
			t.Errorf("shows %s from the hidden side", it.Rel)
		}
	}
}

func TestCompareDoesNotWalkThroughLinks(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "lohn.xls"), []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, e.real("/mnt/user/fotos/tor")); err != nil {
		t.Fatal(err)
	}
	e.write("/mnt/user/backups/tor/anderes.txt", "x")
	var out struct {
		Items []Compared `json:"items"`
	}
	code := e.call("anna", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Recursive: true}, &out)
	if code != http.StatusOK {
		t.Fatalf("compare: %d", code)
	}
	for _, it := range out.Items {
		if it.Name == "lohn.xls" {
			t.Errorf("shows %s from outside the source", it.Rel)
		}
	}
}

func TestPackLeavesOutWhatIsHidden(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.run(KindPack, map[string]any{"sources": []string{"/mnt/user/fotos"}, "dest": "/mnt/user/backups", "name": "alle", "format": "zip"})
	ix, err := archives.Load(e.real("/mnt/user/backups/alle.zip"), archives.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Stat("/fotos/frei.txt"); !ok {
		t.Error("the open file is missing")
	}
	if _, ok := ix.Stat("/fotos/geheim/x.txt"); ok {
		t.Error("the hidden file was packed")
	}
}

func TestAnEncryptedZipDoesNotTakeInWhatALinkLeadsTo(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	e.write("/mnt/user/fotos/frei.txt", "frei")
	outside := filepath.Join(t.TempDir(), "lohn.txt")
	if err := os.WriteFile(outside, []byte("lohn"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, e.real("/mnt/user/fotos/tor.txt")); err != nil {
		t.Fatal(err)
	}
	e.run(KindPack, map[string]any{"sources": []string{"/mnt/user/fotos"}, "dest": "/mnt/user/backups", "name": "alle", "format": "zip", "password": "pw"})
	ix, err := archives.Load(e.real("/mnt/user/backups/alle.zip"), archives.Options{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Stat("/fotos/tor.txt"); ok {
		t.Error("the file behind the link was packed")
	}
}

func TestChecksumLeavesOutWhatIsHidden(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	j := e.run(KindChecksum, map[string]any{"sources": []string{"/mnt/user/fotos"}, "algo": "sha256"})
	sums := result[struct {
		Files []FileSum `json:"files"`
	}](t, j).Files
	for _, s := range sums {
		if filepath.Base(s.Path) == "x.txt" {
			t.Errorf("hashed %s", s.Path)
		}
	}
	if len(sums) == 0 {
		t.Error("the open file is missing")
	}
}

func TestAttrsDoNotReachAProtectedEntryBelow(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.ReadOnly)
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos"}, Mode: "700", Recursive: true}, nil); code != http.StatusForbidden {
		t.Errorf("recursive attrs: %d", code)
	}
}

func TestSetuidIsForAdmins(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/tool", "x")
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/tool"}, Mode: "4755"}, nil); code != http.StatusForbidden {
		t.Errorf("setuid: %d", code)
	}
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/tool"}, Mode: "755"}, nil); code != http.StatusOK {
		t.Errorf("plain mode: %d", code)
	}
}

func TestRenameBatchDoesNotTellWhetherAHiddenNameIsTaken(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.rules.set("/mnt/user/fotos/frei2.txt", ccfs.Hidden)
	var taken, free BatchResult
	e.call("anna", "POST", "/fs/rename/batch", BatchRequest{Paths: []string{"/mnt/user/fotos/frei.txt"}, Names: []string{"geheim"}, DryRun: true}, &taken)
	e.call("anna", "POST", "/fs/rename/batch", BatchRequest{Paths: []string{"/mnt/user/fotos/frei.txt"}, Names: []string{"frei2.txt"}, DryRun: true}, &free)
	if taken.Rows[0].Error != free.Rows[0].Error || taken.Rows[0].Error == "" {
		t.Errorf("taken %q, free %q", taken.Rows[0].Error, free.Rows[0].Error)
	}
}

func TestALinkCannotPointAboveTheScopeOrIntoAReadOnlyFileByHardLink(t *testing.T) {
	e := newEnv(t)
	e.scopes["carl"] = "/fotos"
	e.write("/mnt/user/fotos/a.txt", "a")
	e.write("/mnt/user/backups/b.txt", "b")
	if code := e.call("carl", "POST", "/fs/link", map[string]any{"target": "/mnt/user", "path": "/mnt/user/fotos/tor"}, nil); code != http.StatusForbidden {
		t.Errorf("link above the scope: %d", code)
	}
	e.rules.set("/mnt/user/backups/b.txt", ccfs.ReadOnly)
	if code := e.call("anna", "POST", "/fs/link", map[string]any{"target": "/mnt/user/backups/b.txt", "path": "/mnt/user/fotos/b2.txt", "hard": true}, nil); code != http.StatusForbidden {
		t.Errorf("hard link to a read only file: %d", code)
	}
}

func TestArrowLoopGetsNoFolderARuleNamesSomethingIn(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("ArrowLoop was asked")
	}))
	defer server.Close()
	e.al = ArrowLoopConfig{URL: server.URL, Token: "t"}
	code := e.call("anna", "POST", "/tools/arrowloop", ArrowLoopRequest{Name: "x", Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Direction: "right", Every: "daily", At: "03:15"}, nil)
	if code != http.StatusForbidden {
		t.Errorf("%d", code)
	}
}

// tarWith writes a tar with the entries as name and link target.
func tarWith(t *testing.T, p string, links map[string]string, files map[string]string) {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for name, target := range links {
		if err := w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target, Mode: 0o777}); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		if err := w.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(content)), Mode: 0o666}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
}

func TestUnpackDoesNotChangeAProtectedPlace(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/fest.txt", "alt")
	e.write("/mnt/user/fotos/ordner/fest.txt", "alt")
	e.write("/mnt/user/fotos/ordner/frei.txt", "alt")
	e.rules.set("/mnt/user/fotos/fest.txt", ccfs.ReadOnly)
	e.rules.set("/mnt/user/fotos/ordner/fest.txt", ccfs.ReadOnly)
	tarWith(t, e.real("/mnt/user/downloads/a.tar"), nil, map[string]string{"fest.txt": "neu", "ordner/frei.txt": "neu", "anderes.txt": "neu"})
	j := e.run(KindUnpack, map[string]any{"archives": []string{"/mnt/user/downloads/a.tar"}, "dest": "/mnt/user/fotos", "clash": "replace"})
	if j.State != jobs.Done {
		t.Fatalf("state %s", j.State)
	}
	if got := e.read("/mnt/user/fotos/fest.txt"); got != "alt" {
		t.Errorf("read only file: %q", got)
	}
	if got := e.read("/mnt/user/fotos/ordner/fest.txt"); got != "alt" {
		t.Errorf("read only file in a folder that clashes: %q", got)
	}
	if got := e.read("/mnt/user/fotos/anderes.txt"); got != "neu" {
		t.Errorf("the free member: %q", got)
	}
}

func TestUnpackDoesNotKeepALinkThatLeavesAfterItsPrefixIsCut(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	tarWith(t, e.real("/mnt/user/downloads/a.tar"), map[string]string{"p/q/l": "../../x"}, map[string]string{"p/q/f.txt": "f"})
	e.run(KindUnpack, map[string]any{"archives": []string{"/mnt/user/downloads/a.tar::/p/q"}, "dest": "/mnt/user/fotos"})
	if _, err := os.Lstat(e.real("/mnt/user/fotos/l")); err == nil {
		t.Error("the link was kept")
	}
}

func TestUnpackDoesNotPlantAPartFileOrABin(t *testing.T) {
	e := newEnv(t)
	tarWith(t, e.real("/mnt/user/downloads/a.tar"), nil, map[string]string{".x.cc-part": "p", ".cc-trash/y": "y", "gut.txt": "g"})
	e.run(KindUnpack, map[string]any{"archives": []string{"/mnt/user/downloads/a.tar"}, "dest": "/mnt/user/fotos"})
	if e.exists("/mnt/user/fotos/.x.cc-part") || e.exists("/mnt/user/fotos/.cc-trash/y") {
		t.Error("a reserved name was unpacked")
	}
	if !e.exists("/mnt/user/fotos/gut.txt") {
		t.Error("the plain member is missing")
	}
}

func TestFetchDoesNotWriteThroughALinkPlantedAsItsPartFile(t *testing.T) {
	linksOnly(t)
	e := newEnv(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(outside, e.real("/mnt/user/fotos/.datei.cc-part")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("inhalt")) }))
	defer server.Close()
	saved := fetchClient.Transport
	fetchClient.Transport = &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}
	defer func() { fetchClient.Transport = saved }()
	e.finish(KindFetch, FetchParams{URL: "http://example.org/datei", Dest: "/mnt/user/fotos"})
	if _, err := os.Lstat(outside); err == nil {
		t.Error("the fetch wrote outside the source")
	}
}

func TestCompareSizesAFolderByWhatTheUserSees(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.write("/mnt/user/backups/fotos/frei.txt", "frei")
	var out struct {
		Items []Compared `json:"items"`
	}
	if code := e.call("anna", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user", Right: "/mnt/user/backups", Top: true}, &out); code != http.StatusOK {
		t.Fatalf("compare: %d", code)
	}
	for _, it := range out.Items {
		if it.Name != "fotos" {
			continue
		}
		if it.Left == nil || it.Left.Size != int64(len("frei")) || it.State != Same {
			t.Fatalf("fotos = %+v left %+v", it, it.Left)
		}
		return
	}
	t.Fatal("fotos is missing")
}

func TestSyncCountsWhatItCopies(t *testing.T) {
	e := newEnv(t)
	protectedShare(e, ccfs.Hidden)
	e.write("/mnt/user/backups/.keep", "")
	j := e.run(KindSync, SyncParams{Left: "/mnt/user", Right: "/mnt/user/downloads",
		Steps: []SyncStep{{Rel: "fotos", Act: "copy", To: "R"}}})
	if j.TotalBytes != int64(len("frei")) || j.TotalFiles != 2 {
		t.Fatalf("totals %d bytes, %d files", j.TotalBytes, j.TotalFiles)
	}
}
