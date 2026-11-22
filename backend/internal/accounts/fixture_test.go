package accounts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/app"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

func TestMain(m *testing.M) {
	if err := utils.SetInvalidPasswordHash(); err != nil {
		panic(err)
	}
	os.Exit(testutil.Main(m))
}

// world is a server with the real users table, three sources and the
// accounts API behind the real sign-in. It starts as the version before
// left it, with the admin jdp, and open imports and opens the sign-in.
type world struct {
	t     *testing.T
	base  string
	h     http.Handler
	fs    *ccfs.Resolver
	deps  v1.Deps
	auth  *signin.Auth
	share string
	admin users.User
	// scheme is https once a test gives the server an https address.
	scheme     string
	disk, boot string

	cookies map[string]string
}

const strong = "korrekt-pferd-batterie"

// host is where the tests reach the server, so Origin matches it.
const host = "cc.test"

// newBareWorld is a server of the version before without accounts.
func newBareWorld(t *testing.T) *world {
	t.Helper()
	base := t.TempDir()
	saved := settings.Config
	t.Cleanup(func() { settings.Config = saved })
	settings.Config.Auth.Methods.NoAuth = false
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	settings.Config.Auth.Methods.OidcAuth = settings.OidcConfig{}
	settings.Config.Auth.AdminPassword = ""
	settings.Config.Http.BaseURL = "/"
	settings.Config.Http.ExternalUrl = ""

	share, disk, boot := filepath.Join(base, "user"), filepath.Join(base, "disk1"), filepath.Join(base, "boot")
	for _, d := range []string{"fotos/2026", "filme", "backups/anna", "downloads"} {
		if err := os.MkdirAll(filepath.Join(share, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{disk, boot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(share, "downloads", "film.mkv.partial"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources := []*settings.Source{{Name: "Shares", Path: share}, {Name: "Disk 1", Path: disk}, {Name: "USB-Stick", Path: boot}}
	sources[2].Config.ReadOnly = true
	settings.Config.Server.Sources = sources
	settings.Config.Server.SourceMap = map[string]*settings.Source{}
	settings.Config.Server.NameToSource = map[string]*settings.Source{}
	for _, s := range sources {
		settings.Config.Server.SourceMap[s.Path] = s
		settings.Config.Server.NameToSource[s.Name] = s
	}
	settings.InitializeUserResolvers()

	if _, err := state.Initialize(filepath.Join(base, "state.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.Close() })
	app.MustWireServices(state.Default())
	// The first start makes an admin without a password; the worlds here
	// start from a server whose admin is jdp.
	if first, err := state.GetUserByUsername("admin"); err == nil {
		if err := state.DeleteUser(first.ID); err != nil {
			t.Fatal(err)
		}
	}

	db := state.MainDB()
	rules, err := accounts.Rules(db)
	if err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(rules,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share},
		ccfs.Source{Name: "Disk 1", Mount: "/mnt/disk1", Root: disk, AdminOnly: true},
		ccfs.Source{Name: "USB-Stick", Mount: "/boot", Root: boot, ReadOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, base: base, scheme: "http", fs: fs, deps: v1.Deps{FS: fs, DB: db, Hub: hub.New(hub.Options{})}, share: share, cookies: map[string]string{}}
	w.disk, w.boot = disk, boot
	return w
}

// newWorld is a closed server of the version before, with the admin jdp.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := newBareWorld(t)
	w.admin = w.oldUser("jdp", strong, true, users.BackendScope{Path: w.share, Scope: "/"}, users.BackendScope{Path: w.disk, Scope: "/"}, users.BackendScope{Path: w.boot, Scope: "/"})
	return w
}

// oldUser makes an account as the version before had it, with a bcrypt
// password unless password is empty.
func (w *world) oldUser(name, password string, admin bool, scopes ...users.BackendScope) users.User {
	w.t.Helper()
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true, Configured: true}
	u := users.User{FrontendUser: users.FrontendUser{
		Username: name, LoginMethod: users.LoginMethodPassword,
		Permissions: users.MarkPermissionsConfigured(users.Permissions{Admin: admin, Share: true}),
	}}
	if err := state.CreateUser(&u, password); err != nil {
		w.t.Fatal(err)
	}
	if len(scopes) == 0 {
		scopes = []users.BackendScope{{Path: w.share, Scope: "/"}}
	}
	for i := range scopes {
		scopes[i].Permissions = all
	}
	u.BackendScopes = scopes
	u.Permissions = users.MarkPermissionsConfigured(users.Permissions{Admin: admin, Share: true})
	if err := state.UpdateUser(&u, "", "backendScopes", "permissions"); err != nil {
		w.t.Fatal(err)
	}
	got, err := state.GetUserByID(u.ID)
	if err != nil {
		w.t.Fatal(err)
	}
	return got
}

// open imports the accounts into the sign-in and builds the server.
func (w *world) open() *world {
	w.t.Helper()
	auth, err := accounts.Open(context.Background(), w.deps)
	if err != nil {
		w.t.Fatal(err)
	}
	w.t.Cleanup(accounts.Close)
	w.auth = auth
	r := v1.NewRouter(w.deps, accounts.Authenticate)
	accounts.Routes(r)
	shares.Routes(r)
	api, err := r.Handler()
	if err != nil {
		w.t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, p := range []string{"/api/signin/v1/", "/signin/", "/api/auth/oidc/"} {
		mux.Handle(p, accounts.Handler(auth))
	}
	mux.Handle("/", api)
	w.h = auth.Guard(mux)
	if u, err := state.GetUserByID(w.admin.ID); err == nil {
		w.admin = u
	}
	return w
}

type header = [2]string

// req sends a request from the browser of the server's own page.
func (w *world) req(method, target string, body any, headers ...header) *httptest.ResponseRecorder {
	w.t.Helper()
	var payload string
	switch b := body.(type) {
	case nil:
	case string:
		payload = b
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			w.t.Fatal(err)
		}
		payload = string(raw)
	}
	r := httptest.NewRequest(method, w.scheme+"://"+host+target, strings.NewReader(payload))
	if payload != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Origin", w.scheme+"://"+host)
	for _, h := range headers {
		if h[0] == "Cookie" {
			r.Header.Add("Cookie", h[1])
			continue
		}
		r.Header.Set(h[0], h[1])
	}
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, r)
	return rec
}

// as sends a request as the account with this name, signed in with strong.
func (w *world) as(name, method, target string, body any) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.req(method, target, body, header{"Cookie", w.session(name)})
}

// session signs name in once per test and returns its cookie.
func (w *world) session(name string) string {
	w.t.Helper()
	if c, ok := w.cookies[name]; ok {
		return c
	}
	c, err := w.signIn(name, strong)
	if err != nil {
		w.t.Fatal(err)
	}
	w.cookies[name] = c
	return c
}

// signIn signs in with a password and returns the session cookie.
func (w *world) signIn(name, password string) (string, error) {
	rec := w.req("POST", "/api/signin/v1/login", map[string]string{"name": name, "password": password})
	if rec.Code != http.StatusOK {
		return "", &signInError{rec.Code, rec.Body.String()}
	}
	return sessionCookie(w.t, rec), nil
}

type signInError struct {
	code int
	body string
}

func (e *signInError) Error() string { return http.StatusText(e.code) + ": " + e.body }

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if (c.Name == "cc_session" || c.Name == "__Host-cc-session") && c.MaxAge > 0 {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatalf("no session cookie: %v", rec.Result().Cookies())
	return ""
}

func (w *world) must(want int, rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	w.t.Helper()
	if rec.Code != want {
		w.t.Fatalf("got %d, want %d: %s", rec.Code, want, rec.Body)
	}
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

func detail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[v1.Problem](t, rec).Detail
}

func code(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Code != "" {
		return p.Code
	}
	return p.Error
}

// addUser creates a user through the Users tile and returns its row.
func (w *world) addUser(name, role, scope string, groups ...string) users.User {
	w.t.Helper()
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/users", map[string]any{
		"name": name, "full": strings.ToUpper(name[:1]) + name[1:], "role": role, "scope": scope, "groups": groups, "password": strong,
	}))
	u, err := state.GetUserByUsername(name)
	if err != nil {
		w.t.Fatal(err)
	}
	return u
}

// account is the sign-in's view of the account of u.
func (w *world) account(u users.User) signin.Account {
	w.t.Helper()
	acc, err := w.auth.Account(context.Background(), strconv.FormatUint(u.ID, 10))
	if err != nil {
		w.t.Fatal(err)
	}
	return acc
}


// public gives the server an https address, which passkeys and sign-in
// services need.
func (w *world) public() *world {
	settings.Config.Http.ExternalUrl = "https://" + host
	w.scheme = "https"
	return w
}

// browser keeps the cookies the server sets, as a browser does across the
// steps of a sign-in.
type browser struct {
	w   *world
	jar map[string]string
}

func (w *world) browser() *browser { return &browser{w: w, jar: map[string]string{}} }

func (b *browser) do(method, target string, body any) *httptest.ResponseRecorder {
	b.w.t.Helper()
	var hs []header
	for name, value := range b.jar {
		hs = append(hs, header{"Cookie", name + "=" + value})
	}
	rec := b.w.req(method, target, body, hs...)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(b.jar, c.Name)
			continue
		}
		b.jar[c.Name] = c.Value
	}
	return rec
}

// session is the session cookie the browser holds.
func (b *browser) session() string {
	for _, name := range []string{"cc_session", "__Host-cc-session"} {
		if v, ok := b.jar[name]; ok {
			return name + "=" + v
		}
	}
	return ""
}
