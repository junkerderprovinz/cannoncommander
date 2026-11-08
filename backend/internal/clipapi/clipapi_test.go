package clipapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// fakeApp is the desktop app with a clipboard of its own and a drag that
// only remembers what it was handed.
type fakeApp struct {
	mu      sync.Mutex
	files   []string
	cut     bool
	cleared bool
	dragged []string
}

func (*fakeApp) AutoUpdate() bool         { return true }
func (*fakeApp) SetAutoUpdate(bool) error { return nil }
func (*fakeApp) UpdateReady() string      { return "" }
func (*fakeApp) Restart()                 {}

func (a *fakeApp) ClipboardFiles() ([]string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.files), a.cut, nil
}

func (a *fakeApp) CopyFiles(paths []string, cut bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files, a.cut, a.cleared = slices.Clone(paths), cut, false
	return nil
}

func (a *fakeApp) ClearClipboard() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files, a.cut, a.cleared = nil, false, true
	return nil
}

func (a *fakeApp) DragFiles(paths []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dragged = slices.Clone(paths)
	return nil
}

// plainApp is a desktop app that reaches no clipboard, as on a Mac.
type plainApp struct{}

func (plainApp) AutoUpdate() bool         { return true }
func (plainApp) SetAutoUpdate(bool) error { return nil }
func (plainApp) UpdateReady() string      { return "" }
func (plainApp) Restart()                 {}

func onDesktop(t *testing.T, a desktop.App) {
	t.Helper()
	desktop.Use(a, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
}

type hiddenRules struct{ path string }

func (r hiddenRules) Effect(_ *users.User, p string) ccfs.Effect {
	if p == r.path || strings.HasPrefix(p, r.path+"/") {
		return ccfs.Hidden
	}
	return ccfs.Open
}

type env struct {
	t       *testing.T
	root    string
	outside string
	srv     *httptest.Server
	user    string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	base := t.TempDir()
	e := &env{t: t, root: filepath.Join(base, "home"), outside: filepath.Join(base, "elsewhere"), user: "jdp"}
	for _, d := range []string{filepath.Join(e.root, "work", "sub"), filepath.Join(e.root, "private"), e.outside} {
		if err := os.MkdirAll(d, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		filepath.Join(e.root, "work", "notes.txt"), filepath.Join(e.root, "work", "plan.txt"),
		filepath.Join(e.root, "private", "diary.txt"), filepath.Join(e.outside, "secret.txt"),
	} {
		if err := os.WriteFile(f, []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	fsys, err := ccfs.New(hiddenRules{"/home/jdp/private"}, ccfs.Source{Name: "jdp", Mount: "/home/jdp", Root: e.root})
	if err != nil {
		t.Fatal(err)
	}
	user := func(name string) (*users.User, error) {
		perms := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
		if name == "reader" {
			perms = users.SourceFilePermissions{View: true, Download: true}
		}
		return &users.User{Username: name, BackendScopes: []users.BackendScope{{Path: e.root, Scope: "/", Permissions: perms}}}, nil
	}
	deps := v1.Deps{FS: fsys, Users: user}
	r := v1.NewRouter(deps, func(_ http.ResponseWriter, _ *http.Request) (*users.User, error) { return user(e.user) })
	r.Contract("clipapi", contract)
	bind(r, handlers{deps})
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *env) real(rel string) string { return filepath.Join(e.root, filepath.FromSlash(rel)) }

// do sends a call and returns its status, the code of a problem and the
// body.
func (e *env) do(method, target string, body any) (int, string, []byte) {
	e.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, e.srv.URL+target, bytes.NewReader(raw))
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	var p v1.Problem
	_ = json.Unmarshal(b.Bytes(), &p)
	return resp.StatusCode, p.Code, b.Bytes()
}

func TestTheContainerKeepsItsOwnClipboard(t *testing.T) {
	e := newEnv(t)
	calls := []struct {
		method, target string
		body           any
	}{
		{http.MethodGet, "/clipboard", nil},
		{http.MethodPut, "/clipboard", map[string]any{"paths": []string{"/home/jdp/work/notes.txt"}}},
		{http.MethodPost, "/clipboard/moved", map[string]any{"files": []string{"/home/jdp/work/notes.txt"}}},
		{http.MethodPost, "/dragdrop/in", map[string]any{"paths": []string{e.real("work/notes.txt")}, "to": "/home/jdp"}},
		{http.MethodPost, "/dragdrop/out", map[string]any{"paths": []string{"/home/jdp/work/notes.txt"}}},
	}
	for _, c := range calls {
		if status, code, _ := e.do(c.method, c.target, c.body); status != http.StatusNotFound || code != "desktopOnly" {
			t.Errorf("%s %s: got %d %s", c.method, c.target, status, code)
		}
	}
}

func TestAnAppWithoutClipboardSaysSo(t *testing.T) {
	e := newEnv(t)
	onDesktop(t, plainApp{})
	if status, code, _ := e.do(http.MethodGet, "/clipboard", nil); status != http.StatusNotFound || code != "desktopOnly" {
		t.Fatalf("got %d %s", status, code)
	}
}

type filesJSONForTest struct {
	Files   []string `json:"files"`
	Cut     bool     `json:"cut"`
	Skipped int      `json:"skipped"`
}

func TestFilesCopiedElsewhereArriveAsPathsOfTheSources(t *testing.T) {
	e := newEnv(t)
	app := &fakeApp{cut: true, files: []string{
		e.real("work/notes.txt"), e.real("work/sub"),
		filepath.Join(e.outside, "secret.txt"),
		e.real("private/diary.txt"),
		e.real("work/gone.txt"),
	}}
	onDesktop(t, app)
	status, _, body := e.do(http.MethodGet, "/clipboard", nil)
	if status != http.StatusOK {
		t.Fatalf("got %d %s", status, body)
	}
	var got filesJSONForTest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/jdp/work/notes.txt", "/home/jdp/work/sub"}
	if !slices.Equal(got.Files, want) || !got.Cut || got.Skipped != 3 {
		t.Fatalf("got %+v, want %v cut with 3 skipped", got, want)
	}
}

func TestAnEmptyClipboardHoldsNoFiles(t *testing.T) {
	e := newEnv(t)
	onDesktop(t, &fakeApp{})
	status, _, body := e.do(http.MethodGet, "/clipboard", nil)
	if status != http.StatusOK || strings.TrimSpace(string(body)) != `{"files":[],"cut":false,"skipped":0}` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestCopyingPutsTheFilesOnDiskOnTheClipboard(t *testing.T) {
	e := newEnv(t)
	app := &fakeApp{}
	onDesktop(t, app)
	paths := []string{"/home/jdp/work/notes.txt", "/home/jdp/work/sub"}
	if status, _, body := e.do(http.MethodPut, "/clipboard", map[string]any{"paths": paths, "cut": true}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, body)
	}
	want := []string{e.real("work/notes.txt"), e.real("work/sub")}
	if !slices.Equal(app.files, want) || !app.cut {
		t.Fatalf("clipboard holds %v cut=%v, want %v cut", app.files, app.cut, want)
	}
}

func TestOnlyFilesThePersonMayTakeGoOnTheClipboard(t *testing.T) {
	e := newEnv(t)
	app := &fakeApp{}
	onDesktop(t, app)
	_ = os.Symlink(filepath.Join(e.outside, "secret.txt"), e.real("work/away.txt"))
	refused := []struct {
		path   string
		status int
	}{
		{"/home/jdp/private/diary.txt", http.StatusForbidden},
		{"/home/jdp/work/away.txt", http.StatusForbidden},
		{"/home/jdp/work/notes.txt::/inner.txt", http.StatusBadRequest},
		{"/home", http.StatusForbidden},
	}
	for _, c := range refused {
		status, _, body := e.do(http.MethodPut, "/clipboard", map[string]any{"paths": []string{c.path}})
		if status != c.status {
			t.Errorf("%s: got %d %s, want %d", c.path, status, body, c.status)
		}
	}
	if len(app.files) != 0 {
		t.Fatalf("clipboard holds %v", app.files)
	}
}

func TestCuttingNeedsTheRightToDelete(t *testing.T) {
	e := newEnv(t)
	e.user = "reader"
	app := &fakeApp{}
	onDesktop(t, app)
	paths := []string{"/home/jdp/work/notes.txt"}
	if status, _, _ := e.do(http.MethodPut, "/clipboard", map[string]any{"paths": paths, "cut": true}); status != http.StatusForbidden {
		t.Fatalf("cut: got %d", status)
	}
	if status, _, _ := e.do(http.MethodPut, "/clipboard", map[string]any{"paths": paths}); status != http.StatusNoContent {
		t.Fatalf("copy: got %d", status)
	}
}

func TestAMoveClearsTheClipboardOnlyWhileItHoldsThoseFiles(t *testing.T) {
	e := newEnv(t)
	app := &fakeApp{cut: true, files: []string{e.real("work/notes.txt"), e.real("work/plan.txt")}}
	onDesktop(t, app)
	if status, _, _ := e.do(http.MethodPost, "/clipboard/moved", map[string]any{"files": []string{"/home/jdp/work/notes.txt"}}); status != http.StatusNoContent || app.cleared {
		t.Fatalf("other files: got %d, cleared %v", status, app.cleared)
	}
	moved := []string{"/home/jdp/work/plan.txt", "/home/jdp/work/notes.txt"}
	if status, _, _ := e.do(http.MethodPost, "/clipboard/moved", map[string]any{"files": moved}); status != http.StatusNoContent || !app.cleared {
		t.Fatalf("same files: got %d, cleared %v", status, app.cleared)
	}
	app.files, app.cut, app.cleared = []string{e.real("work/notes.txt")}, false, false
	if _, _, _ = e.do(http.MethodPost, "/clipboard/moved", map[string]any{"files": []string{"/home/jdp/work/notes.txt"}}); app.cleared {
		t.Fatal("cleared files that were copied")
	}
}

func TestDroppedFilesArriveAsPathsOfTheSources(t *testing.T) {
	e := newEnv(t)
	onDesktop(t, &fakeApp{})
	in := map[string]any{"paths": []string{e.real("work/notes.txt"), filepath.Join(e.outside, "secret.txt"), "relative.txt"}, "to": "/home/jdp/work/sub"}
	status, _, body := e.do(http.MethodPost, "/dragdrop/in", in)
	if status != http.StatusOK {
		t.Fatalf("got %d %s", status, body)
	}
	var got struct {
		Files      []string `json:"files"`
		Skipped    int      `json:"skipped"`
		SameVolume bool     `json:"sameVolume"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Files, []string{"/home/jdp/work/notes.txt"}) || got.Skipped != 2 || !got.SameVolume {
		t.Fatalf("got %+v", got)
	}
}

func TestDraggingOutHandsOverTheFilesOnDisk(t *testing.T) {
	e := newEnv(t)
	app := &fakeApp{}
	onDesktop(t, app)
	if status, _, body := e.do(http.MethodPost, "/dragdrop/out", map[string]any{"paths": []string{"/home/jdp/work/notes.txt"}}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, body)
	}
	if !slices.Equal(app.dragged, []string{e.real("work/notes.txt")}) {
		t.Fatalf("dragged %v", app.dragged)
	}
	if status, _, _ := e.do(http.MethodPost, "/dragdrop/out", map[string]any{"paths": []string{"/home/jdp/private/diary.txt"}}); status != http.StatusForbidden {
		t.Fatalf("hidden file: got %d", status)
	}
}

func TestAPathOnDiskFindsItsSource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	sources := []ccfs.Source{{Mount: "/data", Root: root}}
	cases := []struct {
		real, api string
		ok        bool
	}{
		{root, "/data", true},
		{filepath.Join(root, "a", "b.txt"), "/data/a/b.txt", true},
		{filepath.Join(root, "a", "..", "c"), "/data/c", true},
		{root + "2", "", false},
		{filepath.Join(root, "..", "other"), "", false},
		{"data/a", "", false},
	}
	for _, c := range cases {
		api, ok := apiPath(sources, c.real)
		if api != c.api || ok != c.ok {
			t.Errorf("%s: got %q %v, want %q %v", c.real, api, ok, c.api, c.ok)
		}
	}
}
