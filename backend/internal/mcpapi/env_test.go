package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

// The accounts of every test: anna may do everything in her shares, gast may
// look and download, chef is an admin. Ids are random 64-bit numbers in the
// app, and anna's has the high bit set, which SQLite cannot hold as an integer.
const (
	anna = uint64(17795247194615346401)
	gast = uint64(2)
	chef = uint64(3)
)

// rules hide /mnt/user/geheim and make /mnt/user/archiv read only for
// everybody but the admin, as an admin's rule in the app would.
type rules struct{}

func (rules) Effect(u *users.User, p string) ccfs.Effect {
	if u.Permissions.Admin {
		return ccfs.Open
	}
	switch {
	case p == "/mnt/user/geheim" || strings.HasPrefix(p, "/mnt/user/geheim/"):
		return ccfs.Hidden
	case p == "/mnt/user/archiv" || strings.HasPrefix(p, "/mnt/user/archiv/"):
		return ccfs.ReadOnly
	}
	return ccfs.Open
}

// fakeLogin stands for the sign-in system: a request with X-Session has a
// session, one with X-Token came with an API token, and the password is
// "richtig".
type fakeLogin struct{}

func (fakeLogin) SessionHash(r *http.Request) (string, bool) {
	h := r.Header.Get("X-Session")
	return h, h != ""
}
func (fakeLogin) ByAPIToken(r *http.Request) bool { return r.Header.Get("X-Token") != "" }
func (fakeLogin) ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Test-IP"); ip != "" {
		return ip
	}
	return "192.0.2.1"
}
func (fakeLogin) Confirm(_ *http.Request, _ *users.User, password string) error {
	if password != "richtig" {
		return v1.Coded(http.StatusForbidden, "currentPasswordWrong", "the current password is not right")
	}
	return nil
}

// lan stands for the computers a desktop app reaches below /net. It counts
// every question, since each one would go out over the network.
type lan struct {
	mu    sync.Mutex
	asked []string
	root  string
}

func (n *lan) Share(server, share string) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.asked = append(n.asked, server+"/"+share)
	return n.root, server == "nas" && share == "daten"
}

func (n *lan) Shares(server string) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.asked = append(n.asked, server)
	if server == "nas" {
		return []string{"daten"}
	}
	return nil
}

func (n *lan) questions() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.asked...)
}

type env struct {
	t     *testing.T
	root  string
	net   *lan
	deps  v1.Deps
	srv   *Server
	web   *httptest.Server
	mu    sync.Mutex
	users map[uint64]*users.User
	now   time.Time
	// signIn is whether the server asks people to sign in.
	signIn bool
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"filme", "geheim", "archiv", "fotos"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	// A source only the admin has in scope, and a share another computer
	// offers below /net.
	extern, share := t.TempDir(), t.TempDir()
	e := &env{t: t, root: root, users: map[uint64]*users.User{}, now: time.Unix(1_800_000_000, 0), signIn: true, net: &lan{root: share}}
	full := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	look := users.SourceFilePermissions{View: true, Download: true}
	for id, spec := range map[uint64]struct {
		name  string
		perms users.SourceFilePermissions
		admin bool
	}{anna: {"anna", full, false}, gast: {"gast", look, false}, chef: {"chef", full, true}} {
		u := &users.User{ID: id, Username: spec.name, BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: spec.perms}}}
		u.Permissions.Admin = spec.admin
		if spec.admin {
			u.BackendScopes = append(u.BackendScopes, users.BackendScope{Path: extern, Scope: "/", Permissions: full})
		}
		u.LoginMethod = users.LoginMethodPassword
		e.users[id] = u
	}
	fsys, err := ccfs.New(rules{}, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root}, ccfs.Source{Name: "Extern", Mount: "/mnt/disks/extern", Root: extern})
	if err != nil {
		t.Fatal(err)
	}
	fsys = fsys.WithNetwork(e.net)
	e.deps = v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t), Users: e.byName}
	e.srv = newServer(e.deps)
	e.srv.login = fakeLogin{}
	e.srv.user = e.byID
	e.srv.pepper = func() []byte { return []byte("pepper-for-the-tests") }
	e.srv.signIn = func() bool { return e.signIn }
	e.srv.now = e.clock
	e.serve()
	return e
}

func (e *env) clock() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *env) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

func (e *env) byID(id uint64) (*users.User, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	u, ok := e.users[id]
	if !ok {
		return nil, errors.New("no such user")
	}
	c := *u
	return &c, nil
}

func (e *env) byName(name string) (*users.User, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, u := range e.users {
		if u.Username == name {
			c := *u
			return &c, nil
		}
	}
	return nil, errors.New("no such user")
}

// serve builds /api/v1 with the packages the tools reach, behind the same
// authenticator the server uses, and the endpoint next to it.
func (e *env) serve() {
	t := e.t
	session := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		name := r.Header.Get("X-User")
		if name == "" {
			return nil, v1.Coded(http.StatusUnauthorized, "signInNeeded", "login required")
		}
		return e.byName(name)
	}
	r := v1.NewRouter(e.deps, Authenticate(session))
	fsapi.Routes(r)
	fileops.Routes(r)
	jobs.Routes(r)
	undo.Routes(r)
	trash.Routes(r)
	search.Routes(r)
	routesOf(r, e.srv)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", h))
	e.srv.Mount(mux, "/", h)
	e.web = httptest.NewServer(mux)
	t.Cleanup(func() {
		e.web.Close()
		if m := jobs.For(e.deps); m != nil {
			m.Close()
		}
		if s := trash.For(e.deps); s != nil {
			s.Close()
		}
		for _, name := range []string{"jobs", "trash", "fileops", "undo", "mcp"} {
			v1.Forget(e.deps, name)
		}
	})
}

func (e *env) real(api string) string {
	return filepath.Join(e.root, filepath.FromSlash(strings.TrimPrefix(api, "/mnt/user")))
}

func (e *env) write(api, content string) {
	e.t.Helper()
	p := e.real(api)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) exists(api string) bool {
	_, err := os.Lstat(e.real(api))
	return err == nil
}

// api sends a request to /api/v1 as user with a session, and decodes the
// answer into out.
func (e *env) api(user, method, target string, body, out any, headers ...string) int {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.web.URL+"/api/v1"+target, rd)
	req.Header.Set("X-User", user)
	req.Header.Set("X-Session", "session-of-"+user)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		switch {
		case headers[i] == "Host":
			req.Host = headers[i+1]
		case headers[i+1] == "":
			req.Header.Del(headers[i])
		default:
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		raw, _ := io.ReadAll(resp.Body)
		_ = json.Unmarshal(raw, out)
	}
	return resp.StatusCode
}

// key makes a key for user through the API and returns it with its id.
func (e *env) key(user string, canWrite bool) (key, id string) {
	e.t.Helper()
	var out struct {
		Key  string  `json:"key"`
		Item KeyView `json:"item"`
	}
	if code := e.api(user, http.MethodPost, "/mcp/keys", map[string]any{"label": user + " " + randomLabel(), "canWrite": canWrite, "password": "richtig"}, &out); code != http.StatusCreated {
		e.t.Fatalf("make a key for %s: %d", user, code)
	}
	return out.Key, out.Item.ID
}

var labelSeq struct {
	sync.Mutex
	n int
}

func randomLabel() string {
	labelSeq.Lock()
	defer labelSeq.Unlock()
	labelSeq.n++
	return "key " + string(rune('a'+labelSeq.n%26)) + strings.Repeat("x", labelSeq.n/26)
}

// keyTransport puts the key on every request the SDK client sends, which is
// what a client configured with a header does.
type keyTransport struct{ key string }

func (t keyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+t.key)
	return http.DefaultTransport.RoundTrip(r)
}

// client connects the official SDK client with key, the way every client in
// the field talks to the endpoint.
func (e *env) client(key string) *mcp.ClientSession {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "cannoncommander-tests", Version: "1"}, nil)
	s, err := c.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             e.web.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: keyTransport{key: key}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		e.t.Fatalf("connect: %v", err)
	}
	e.t.Cleanup(func() { _ = s.Close() })
	return s
}

// tool calls name and returns its structured answer, or the code of its
// error.
func (e *env) tool(s *mcp.ClientSession, name string, args map[string]any) (map[string]any, string) {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		e.t.Fatalf("%s: %v", name, err)
	}
	out, _ := res.StructuredContent.(map[string]any)
	if res.IsError {
		body, _ := out["error"].(map[string]any)
		code, _ := body["code"].(string)
		if code == "" {
			code = "error"
		}
		return out, code
	}
	return out, ""
}

// waitJob waits until the job ended and returns its state.
func (e *env) waitJob(user, id string) string {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var j apiJob
		e.api(user, http.MethodGet, "/jobs/"+id, nil, &j)
		if j.State != "" && !j.open() {
			return j.State
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.t.Fatalf("job %s did not end", id)
	return ""
}

// raw posts body to the endpoint with the given headers and returns the
// status and the answer.
func (e *env) raw(method, body string, headers ...string) (int, string, http.Header) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.web.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw), resp.Header
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

const initBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
