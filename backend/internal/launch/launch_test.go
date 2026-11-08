package launch

import (
	"bytes"
	"encoding/json"
	"errors"
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

// fakeSystem asks before every file named run.sh and remembers what it
// started.
type fakeSystem struct {
	mu      sync.Mutex
	fail    error
	opened  []string
	started []string
}

func (s *fakeSystem) asks(file string) bool { return filepath.Base(file) == "run.sh" }

func (s *fakeSystem) open(file string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = append(s.opened, file)
	return s.fail
}

func (s *fakeSystem) run(file string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = append(s.started, file)
	return s.fail
}

type fakeApp struct{}

func (fakeApp) AutoUpdate() bool         { return true }
func (fakeApp) SetAutoUpdate(bool) error { return nil }
func (fakeApp) UpdateReady() string      { return "" }
func (fakeApp) Restart()                 {}

func onDesktop(t *testing.T) {
	t.Helper()
	desktop.Use(fakeApp{}, desktop.Options{})
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
	t    *testing.T
	root string
	srv  *httptest.Server
	sys  *fakeSystem
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"work/sub", "private"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"work/notes.txt", "work/run.sh", "private/diary.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	fsys, err := ccfs.New(hiddenRules{"/mnt/user/private"}, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, root: root, sys: &fakeSystem{}}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	user := func(name string) (*users.User, error) {
		return &users.User{Username: name, BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: all}}}, nil
	}
	deps := v1.Deps{FS: fsys, Users: user}
	r := v1.NewRouter(deps, func(_ http.ResponseWriter, _ *http.Request) (*users.User, error) { return user("jdp") })
	r.Contract("launch", contract)
	bind(r, &handlers{deps: deps, sys: e.sys})
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(e.srv.Close)
	return e
}

// launch posts to /launch and returns the status and the problem's code.
func (e *env) launch(path string, run bool) (int, string) {
	e.t.Helper()
	raw, _ := json.Marshal(map[string]any{"path": path, "run": run})
	resp, err := http.Post(e.srv.URL+"/launch", "application/json", bytes.NewReader(raw))
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var p v1.Problem
	_ = json.NewDecoder(resp.Body).Decode(&p)
	return resp.StatusCode, p.Code
}

func TestTheContainerLaunchesNothing(t *testing.T) {
	e := newEnv(t)
	if status, code := e.launch("/mnt/user/work/notes.txt", false); status != http.StatusNotFound || code != "desktopOnly" {
		t.Fatalf("got %d %s", status, code)
	}
	if len(e.sys.opened) != 0 {
		t.Fatalf("opened %v", e.sys.opened)
	}
}

func TestAFileOpensInItsProgram(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	if status, code := e.launch("/mnt/user/work/notes.txt", false); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
	if want := []string{filepath.Join(e.root, "work", "notes.txt")}; !slices.Equal(e.sys.opened, want) {
		t.Fatalf("opened %v", e.sys.opened)
	}
}

func TestAProgramStartsOnlyOnceThePersonSaidSo(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	if status, code := e.launch("/mnt/user/work/run.sh", false); status != http.StatusConflict || code != "askRun" {
		t.Fatalf("got %d %s", status, code)
	}
	if len(e.sys.started)+len(e.sys.opened) != 0 {
		t.Fatalf("started %v before the answer", e.sys.started)
	}
	if status, code := e.launch("/mnt/user/work/run.sh", true); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
	if want := []string{filepath.Join(e.root, "work", "run.sh")}; !slices.Equal(e.sys.started, want) || len(e.sys.opened) != 0 {
		t.Fatalf("started %v, opened %v", e.sys.started, e.sys.opened)
	}
}

func TestOnlyAFileTheRulesShowOpens(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	for path, want := range map[string]int{
		"/mnt/user/private/diary.txt": http.StatusForbidden,
		"/mnt/user/work/gone.txt":     http.StatusNotFound,
		"/mnt/user/work/sub":          http.StatusBadRequest,
	} {
		if status, code := e.launch(path, false); status != want {
			t.Errorf("%s: got %d %s", path, status, code)
		}
	}
	if len(e.sys.opened) != 0 {
		t.Fatalf("opened %v", e.sys.opened)
	}
}

func TestAProgramThatDoesNotStartSaysSo(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	e.sys.fail = errors.New("no program")
	if status, code := e.launch("/mnt/user/work/notes.txt", false); status != http.StatusBadGateway || code != "notStarted" {
		t.Fatalf("got %d %s", status, code)
	}
}
