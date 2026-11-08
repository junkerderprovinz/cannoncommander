package openwith

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// fakeSystem offers the same programs for every file and remembers what it
// was asked to do.
type fakeSystem struct {
	mu     sync.Mutex
	offer  []app
	all    []app
	ownDlg bool
	fail   error
	opened [][2]string
	icons  map[string]int
	chose  chan string
}

func (s *fakeSystem) apps(string) ([]app, error) { return s.offer, nil }
func (s *fakeSystem) every() ([]app, error)      { return s.all, nil }
func (s *fakeSystem) dialog() bool               { return s.ownDlg }

func (s *fakeSystem) icon(where string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.icons[where]++
	if where == "broken" {
		return "", errNoIcon
	}
	return "data:image/png;base64," + where, nil
}

func (s *fakeSystem) open(file, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return s.fail
	}
	s.opened = append(s.opened, [2]string{file, id})
	return nil
}

func (s *fakeSystem) choose(file string) error {
	s.chose <- file
	return nil
}

func (s *fakeSystem) started() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]string(nil), s.opened...)
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
	base := t.TempDir()
	root, outside := filepath.Join(base, "shares"), filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(root, "work", "sub"), filepath.Join(root, "private"), outside} {
		if err := os.MkdirAll(d, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(root, "work", "notes.txt"), filepath.Join(root, "private", "diary.txt"), filepath.Join(outside, "secret.txt")} {
		if err := os.WriteFile(f, []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	// A link that leads out of the source, where no file of it may be opened.
	_ = os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "work", "away.txt"))
	fsys, err := ccfs.New(hiddenRules{"/mnt/user/private"}, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, root: root, sys: &fakeSystem{
		offer: []app{{id: "editor", name: "Editor", icon: "ed", def: true}, {id: "code", name: "Code", icon: "broken"}},
		all:   []app{{id: "calc", name: "Calculator"}},
		icons: map[string]int{},
		chose: make(chan string, 1),
	}}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	user := func(name string) (*users.User, error) {
		return &users.User{Username: name, BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: all}}}, nil
	}
	deps := v1.Deps{FS: fsys, Users: user}
	r := v1.NewRouter(deps, func(_ http.ResponseWriter, _ *http.Request) (*users.User, error) { return user("jdp") })
	r.Contract("openwith", contract)
	bind(r, newHandlers(deps, e.sys))
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(e.srv.Close)
	return e
}

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

func (e *env) list(path string, every bool) (int, string, listJSON) {
	e.t.Helper()
	q := url.Values{"path": {path}}
	if every {
		q.Set("every", "true")
	}
	status, code, body := e.do(http.MethodGet, "/openwith?"+q.Encode(), nil)
	var l listJSON
	_ = json.Unmarshal(body, &l)
	return status, code, l
}

func TestTheContainerOpensNothingWithAnotherProgram(t *testing.T) {
	e := newEnv(t)
	calls := []struct{ method, target string }{
		{http.MethodGet, "/openwith?path=/mnt/user/work/notes.txt"},
		{http.MethodPost, "/openwith"},
		{http.MethodPost, "/openwith/dialog"},
	}
	for _, c := range calls {
		if status, code, _ := e.do(c.method, c.target, map[string]string{"path": "/mnt/user/work/notes.txt", "app": "editor"}); status != http.StatusNotFound || code != "desktopOnly" {
			t.Errorf("%s %s: got %d %s", c.method, c.target, status, code)
		}
	}
	if got := e.sys.started(); len(got) != 0 {
		t.Fatalf("opened %v", got)
	}
}

func TestTheProgramsComeWithTheirIcons(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	status, code, l := e.list("/mnt/user/work/notes.txt", false)
	if status != http.StatusOK {
		t.Fatalf("got %d %s", status, code)
	}
	want := []appJSON{{ID: "editor", Name: "Editor", Icon: "data:image/png;base64,ed", Default: true}, {ID: "code", Name: "Code"}}
	if len(l.Apps) != len(want) || l.Apps[0] != want[0] || l.Apps[1] != want[1] {
		t.Fatalf("got %+v", l.Apps)
	}
	if l.Dialog {
		t.Fatal("the fake has no dialog")
	}
}

func TestEachIconIsMadeOnce(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	for range 3 {
		e.list("/mnt/user/work/notes.txt", false)
	}
	if e.sys.icons["ed"] != 1 || e.sys.icons["broken"] != 1 {
		t.Fatalf("made %v", e.sys.icons)
	}
}

func TestOnlyAFileTheUserMayReachIsOffered(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	cases := []struct {
		name, path string
		status     int
	}{
		{"a folder", "/mnt/user/work/sub", http.StatusBadRequest},
		{"a hidden file", "/mnt/user/private/diary.txt", http.StatusForbidden},
		{"no file at all", "/mnt/user/work/gone.txt", http.StatusNotFound},
		{"above the sources", "/mnt", http.StatusForbidden},
		{"a path that climbs out", "/mnt/user/work/../../../etc/passwd", http.StatusBadRequest},
	}
	if _, err := os.Lstat(filepath.Join(e.root, "work", "away.txt")); err == nil {
		cases = append(cases, struct {
			name, path string
			status     int
		}{"a link out of the source", "/mnt/user/work/away.txt", http.StatusForbidden})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, code, _ := e.list(c.path, false)
			if status != c.status {
				t.Errorf("list: got %d %s", status, code)
			}
			status, code, _ = e.do(http.MethodPost, "/openwith", map[string]string{"path": c.path, "app": "editor"})
			if status != c.status {
				t.Errorf("open: got %d %s", status, code)
			}
		})
	}
	if got := e.sys.started(); len(got) != 0 {
		t.Fatalf("opened %v", got)
	}
}

func TestAFileOpensWithAProgramOfTheList(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	if status, code, _ := e.do(http.MethodPost, "/openwith", map[string]string{"path": "/mnt/user/work/notes.txt", "app": "code"}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
	got := e.sys.started()
	if len(got) != 1 || got[0][0] != filepath.Join(e.root, "work", "notes.txt") || got[0][1] != "code" {
		t.Fatalf("opened %v", got)
	}
}

func TestAProgramOffTheListIsRefused(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	for _, id := range []string{"calc", "", `C:\Windows\System32\cmd.exe`, "/bin/sh"} {
		if status, code, _ := e.do(http.MethodPost, "/openwith", map[string]string{"path": "/mnt/user/work/notes.txt", "app": id}); status != http.StatusBadRequest || code != "unknownApp" {
			t.Errorf("%q: got %d %s", id, status, code)
		}
	}
	if got := e.sys.started(); len(got) != 0 {
		t.Fatalf("opened %v", got)
	}
}

func TestWithoutADialogEveryProgramCanBePicked(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	_, _, l := e.list("/mnt/user/work/notes.txt", true)
	if len(l.Apps) != 1 || l.Apps[0].ID != "calc" {
		t.Fatalf("got %+v", l.Apps)
	}
	if status, code, _ := e.do(http.MethodPost, "/openwith", map[string]any{"path": "/mnt/user/work/notes.txt", "app": "calc", "every": true}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
	if status, code, _ := e.do(http.MethodPost, "/openwith/dialog", map[string]string{"path": "/mnt/user/work/notes.txt"}); status != http.StatusNotFound || code != "noAppDialog" {
		t.Fatalf("dialog: got %d %s", status, code)
	}
}

func TestWithADialogTheSystemPicks(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	e.sys.ownDlg = true
	if status, code, _ := e.list("/mnt/user/work/notes.txt", true); status != http.StatusNotFound || code != "noAppDialog" {
		t.Fatalf("every: got %d %s", status, code)
	}
	if status, code, _ := e.do(http.MethodPost, "/openwith", map[string]any{"path": "/mnt/user/work/notes.txt", "app": "calc", "every": true}); status != http.StatusNotFound || code != "noAppDialog" {
		t.Fatalf("open: got %d %s", status, code)
	}
	if status, code, _ := e.do(http.MethodPost, "/openwith/dialog", map[string]string{"path": "/mnt/user/work/notes.txt"}); status != http.StatusNoContent {
		t.Fatalf("dialog: got %d %s", status, code)
	}
	select {
	case file := <-e.sys.chose:
		if file != filepath.Join(e.root, "work", "notes.txt") {
			t.Fatalf("the dialog got %s", file)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no dialog")
	}
	if status, code, _ := e.do(http.MethodPost, "/openwith/dialog", map[string]string{"path": "/mnt/user/work/sub"}); status != http.StatusBadRequest || code != "notAFile" {
		t.Fatalf("a folder: got %d %s", status, code)
	}
}

func TestAProgramThatDoesNotStartIsNamed(t *testing.T) {
	onDesktop(t)
	e := newEnv(t)
	e.sys.fail = errors.New("gone")
	status, code, body := e.do(http.MethodPost, "/openwith", map[string]string{"path": "/mnt/user/work/notes.txt", "app": "editor"})
	if status != http.StatusBadGateway || code != "appNotStarted" || !strings.Contains(string(body), `"app":"Editor"`) {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestTheDefaultComesFirst(t *testing.T) {
	got := defaultFirst([]app{{id: "a"}, {id: "b"}, {id: "c", def: true}, {id: "d", def: true}})
	ids := ""
	for _, a := range got {
		ids += a.id
		if a.def != (a.id == "c") {
			t.Fatalf("%s marked %v", a.id, a.def)
		}
	}
	if ids != "cabd" {
		t.Fatalf("got %s", ids)
	}
}

func TestAnIconFillsItsSquare(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for i := range wide.Pix {
		wide.Pix[i] = 0xff
	}
	u, err := pngIcon(wide)
	if err != nil {
		t.Fatal(err)
	}
	img := decodeDataURL(t, u)
	if img.Bounds() != image.Rect(0, 0, iconSize, iconSize) {
		t.Fatalf("got %v", img.Bounds())
	}
	// The wide picture keeps its shape: clear above and below, solid across.
	if _, _, _, a := img.At(iconSize/2, 0).RGBA(); a != 0 {
		t.Fatal("the top is not clear")
	}
	if _, _, _, a := img.At(0, iconSize/2).RGBA(); a == 0 {
		t.Fatal("the middle is clear")
	}
}

func decodeDataURL(t *testing.T, u string) image.Image {
	t.Helper()
	b64, ok := strings.CutPrefix(u, "data:image/png;base64,")
	if !ok {
		t.Fatalf("not a PNG: %.40s", u)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}
