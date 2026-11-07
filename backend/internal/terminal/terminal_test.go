package terminal

import (
	"bytes"
	"encoding/json"
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
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

// fakeGate stands in for the web server: a session is the cookie "s", an API
// token is a bearer header, and a request whose Origin names another host is
// cross-origin.
type fakeGate struct {
	mu      sync.Mutex
	revoked map[string]bool
	open    bool
}

func (g *fakeGate) SameOrigin(r *http.Request) error {
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || u.Host != r.Host {
			return v1.Coded(http.StatusForbidden, "crossOrigin", "cross-origin request")
		}
	}
	return nil
}

func (g *fakeGate) Session(r *http.Request) (string, bool) {
	c, err := r.Cookie("s")
	if err != nil {
		return "", false
	}
	return c.Value, true
}

func (g *fakeGate) Alive(hash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.revoked[hash]
}

func (g *fakeGate) APIToken(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func (g *fakeGate) Open() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.open
}

func (g *fakeGate) revoke(hash string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.revoked == nil {
		g.revoked = map[string]bool{}
	}
	g.revoked[hash] = true
}

type env struct {
	t    *testing.T
	root string
	srv  *httptest.Server
	gate *fakeGate
	h    *handlers

	mu      sync.Mutex
	admins  map[string]bool
	entries []activitydb.Entry
}

type hiddenRules struct{ path string }

func (r hiddenRules) Effect(_ *users.User, p string) ccfs.Effect {
	if p == r.path || strings.HasPrefix(p, r.path+"/") {
		return ccfs.Hidden
	}
	return ccfs.Open
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"work", "private", filepath.Join("appdata", "cannoncommander")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "work", "notes.txt"), []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	fsys, err := ccfs.New(hiddenRules{"/mnt/user/private"}, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	fsys = fsys.WithDataFolder(filepath.Join(root, "appdata", "cannoncommander"))
	e := &env{t: t, root: root, gate: &fakeGate{}, admins: map[string]bool{"jdp": true}}
	deps := v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t), Users: e.user}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		return e.user(r.Header.Get("X-User"))
	}
	r := v1.NewRouter(deps, auth)
	system.Routes(r)
	e.h = newHandlers(deps, e.gate)
	e.h.record = func(_ *http.Request, _ *users.User, entry activitydb.Entry) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.entries = append(e.entries, entry)
	}
	r.Contract("terminal", contract)
	bind(r, e.h)
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(func() {
		e.h.pool.Close()
		e.srv.Close()
		v1.Forget(deps, "system.settings")
	})
	return e
}

func (e *env) user(name string) (*users.User, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if name == "" {
		return nil, v1.Coded(http.StatusUnauthorized, "signInNeeded", "login required")
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	return &users.User{
		ID:            uint64(len(name)),
		Username:      name,
		Permissions:   users.Permissions{Admin: e.admins[name]},
		BackendScopes: []users.BackendScope{{Path: e.root, Scope: "/", Permissions: all}},
	}, nil
}

func (e *env) demote(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.admins[name] = false
}

func (e *env) recorded() []activitydb.Entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]activitydb.Entry(nil), e.entries...)
}

// request builds a call as the browser of an admin with a session makes it.
func (e *env) request(method, target string, body any) *http.Request {
	e.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, e.srv.URL+target, bytes.NewReader(raw))
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("X-User", "jdp")
	req.AddCookie(&http.Cookie{Name: "s", Value: "session-1"})
	return req
}

func (e *env) do(req *http.Request) (int, string) {
	e.t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var p v1.Problem
	_ = json.NewDecoder(resp.Body).Decode(&p)
	return resp.StatusCode, p.Code
}

func (e *env) check(path string) (int, string) {
	e.t.Helper()
	return e.do(e.request(http.MethodGet, "/terminal?path="+url.QueryEscape(path), nil))
}

func (e *env) switchOn(on bool) {
	e.t.Helper()
	if status, code := e.do(e.request(http.MethodPut, "/admin/app", map[string]bool{"terminal": on})); status != http.StatusOK {
		e.t.Fatalf("saving the switch: %d %s", status, code)
	}
}

func TestOffUntilAnAdminSwitchesItOn(t *testing.T) {
	e := newEnv(t)
	if status, code := e.check("/mnt/user/work"); status != http.StatusForbidden || code != "terminalOff" {
		t.Fatalf("fresh server: %d %s", status, code)
	}
	e.switchOn(true)
	if status, code := e.check("/mnt/user/work"); status != http.StatusNoContent {
		t.Fatalf("switched on: %d %s", status, code)
	}
	e.switchOn(false)
	if status, code := e.check("/mnt/user/work"); status != http.StatusForbidden || code != "terminalOff" {
		t.Fatalf("switched off again: %d %s", status, code)
	}
}

func TestOnlyAdminsGetATerminal(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	req := e.request(http.MethodGet, "/terminal?path=/mnt/user/work", nil)
	req.Header.Set("X-User", "anna")
	if status, code := e.do(req); status != http.StatusForbidden || code != "adminsOnly" {
		t.Fatalf("got %d %s", status, code)
	}
}

func TestWithoutSignInItStaysOffUntilSwitchedOn(t *testing.T) {
	e := newEnv(t)
	e.gate.open = true
	plain := func() (int, string) {
		req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/terminal?path=/mnt/user/work", nil)
		req.Header.Set("X-User", "jdp")
		return e.do(req)
	}
	if status, code := plain(); status != http.StatusForbidden || code != "terminalOff" {
		t.Fatalf("fresh server: %d %s", status, code)
	}
	e.switchOn(true)
	if status, code := plain(); status != http.StatusNoContent {
		t.Fatalf("switched on: %d %s", status, code)
	}
}

func TestASignedInServerWantsASession(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/terminal?path=/mnt/user/work", nil)
	req.Header.Set("X-User", "jdp")
	if status, code := e.do(req); status != http.StatusUnauthorized || code != "signInNeeded" {
		t.Fatalf("got %d %s", status, code)
	}
}

func TestAPITokensAreRefused(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	req := e.request(http.MethodGet, "/terminal?path=/mnt/user/work", nil)
	req.Header.Set("Authorization", "Bearer a.b.c")
	if status, code := e.do(req); status != http.StatusForbidden || code != "terminalToken" {
		t.Fatalf("got %d %s", status, code)
	}
}

func TestAnotherOriginIsRefused(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	req := e.request(http.MethodGet, "/terminal?path=/mnt/user/work", nil)
	req.Header.Set("Origin", "http://evil.example")
	if status, code := e.do(req); status != http.StatusForbidden || code != "crossOrigin" {
		t.Fatalf("got %d %s", status, code)
	}
}

func TestOnlyARealFolderTheUserMayOpen(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	for _, c := range []struct {
		path   string
		status int
		code   string
	}{
		{"", http.StatusBadRequest, "badPath"},
		{"work", http.StatusBadRequest, "badPath"},
		{"/mnt/user/gone", http.StatusNotFound, "notFound"},
		{"/mnt/user/work/notes.txt", http.StatusBadRequest, "notAFolder"},
		{"/mnt", http.StatusBadRequest, "notAFolder"},
		{"/mnt/user/private", http.StatusForbidden, "denied"},
		{"/mnt/user/work/../private", http.StatusBadRequest, "badPath"},
		{"/mnt/user/appdata/cannoncommander", http.StatusNotFound, "notFound"},
		{"/mnt/user/work", http.StatusNoContent, ""},
	} {
		status, code := e.check(c.path)
		if status != c.status || code != c.code {
			t.Errorf("%q: got %d %s, want %d %s", c.path, status, code, c.status, c.code)
		}
	}
}

func TestALinkIntoTheDataFolderIsRefused(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	if err := os.Symlink(filepath.Join(e.root, "appdata", "cannoncommander"), filepath.Join(e.root, "work", "data")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if status, _ := e.check("/mnt/user/work/data"); status == http.StatusNoContent {
		t.Fatal("a link into the data folder opened")
	}
}

func TestSizesStayInBounds(t *testing.T) {
	for _, c := range []struct {
		cols, rows string
		want       size
	}{
		{"", "", size{80, 24}},
		{"120", "40", size{120, 40}},
		{"0", "-3", size{80, 24}},
		{"100000", "x", size{1000, 24}},
	} {
		got := sizeOf(c.cols, c.rows)
		if got != c.want {
			t.Errorf("%s x %s: got %v, want %v", c.cols, c.rows, got, c.want)
		}
	}
}

func TestPoolKeepsToTheLimitPerUser(t *testing.T) {
	p := newPool(2)
	a, b, c := newSession("jdp", ""), newSession("jdp", ""), newSession("jdp", "")
	if err := p.add(a); err != nil {
		t.Fatal(err)
	}
	if err := p.add(b); err != nil {
		t.Fatal(err)
	}
	if err := p.add(c); err == nil {
		t.Fatal("a third session got in")
	}
	anna := newSession("anna", "")
	if err := p.add(anna); err != nil {
		t.Fatal("another user is held back by jdp's sessions:", err)
	}
	p.remove(a)
	if err := p.add(c); err != nil {
		t.Fatal("a freed place stays taken:", err)
	}
	for _, s := range []*session{b, c, anna} {
		p.remove(s)
	}
	p.Close()
	if err := p.add(newSession("tom", "")); err == nil {
		t.Fatal("a closed pool takes new sessions")
	}
}

func TestEndingTwiceKeepsTheFirstReason(t *testing.T) {
	s := newSession("jdp", "")
	s.end(closeIdle, "idle")
	s.end(closeNormal, "exited")
	select {
	case <-s.ending:
	case <-time.After(time.Second):
		t.Fatal("ending was not signalled")
	}
	if s.code != closeIdle || s.reason != "idle" {
		t.Fatalf("got %d %s", s.code, s.reason)
	}
}
