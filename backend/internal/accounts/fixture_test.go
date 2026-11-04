package accounts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/app"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestMain(m *testing.M) {
	if err := utils.SetInvalidPasswordHash(); err != nil {
		panic(err)
	}
	os.Exit(testutil.Main(m))
}

// world is a server with the real login store, three sources and the
// accounts API. Requests name their user in X-User, or bring an API token
// in X-Api.
type world struct {
	t     *testing.T
	h     http.Handler
	fs    *ccfs.Resolver
	deps  v1.Deps
	login *fakeLogin
	share string
	admin users.User
}

const strong = "korrekt-pferd-batterie"

func newWorld(t *testing.T) *world {
	t.Helper()
	base := t.TempDir()
	saved := settings.Config
	t.Cleanup(func() { settings.Config = saved })

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

	db := state.MainDB()
	gate, err := accounts.Gate(state.Default(), db)
	if err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(gate,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share},
		ccfs.Source{Name: "Disk 1", Mount: "/mnt/disk1", Root: disk, AdminOnly: true},
		ccfs.Source{Name: "USB-Stick", Mount: "/boot", Root: boot, ReadOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := accounts.RecoveryCodes(db)
	if err != nil {
		t.Fatal(err)
	}
	auth.UseRecoveryCodes(codes)
	t.Cleanup(func() { auth.UseRecoveryCodes(nil) })

	login := &fakeLogin{dead: map[string]bool{}}
	accounts.UseLogin(login)
	authenticate := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if raw := r.Header.Get("X-Api"); raw != "" {
			id, isSession, ok := state.HashedTokenOwner(raw)
			if !ok || isSession || state.IsTokenRevoked(raw) {
				return nil, v1.Errorf(http.StatusUnauthorized, "login required")
			}
			u, err := state.GetUserByID(id)
			return &u, err
		}
		u, err := state.GetUserByUsername(r.Header.Get("X-User"))
		if err != nil {
			return nil, v1.Errorf(http.StatusUnauthorized, "login required")
		}
		return &u, nil
	}
	deps := v1.Deps{FS: fs, DB: db, Hub: hub.New(hub.Options{})}
	r := v1.NewRouter(deps, accounts.Track(db, authenticate))
	accounts.Routes(r)
	shares.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, h: h, fs: fs, deps: deps, login: login, share: share}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true, Configured: true}
	admin := users.User{FrontendUser: users.FrontendUser{
		Username:    "jdp",
		LoginMethod: users.LoginMethodPassword,
		Permissions: users.MarkPermissionsConfigured(users.Permissions{Admin: true}),
	}}
	if err := state.CreateUser(&admin, strong); err != nil {
		t.Fatal(err)
	}
	admin.BackendScopes = []users.BackendScope{{Path: share, Scope: "/", Permissions: all}, {Path: disk, Scope: "/", Permissions: all}, {Path: boot, Scope: "/", Permissions: all}}
	admin.Permissions = users.MarkPermissionsConfigured(users.Permissions{Admin: true})
	if err := state.UpdateUser(&admin, "", "backendScopes", "permissions"); err != nil {
		t.Fatal(err)
	}
	if w.admin, err = state.GetUserByID(admin.ID); err != nil {
		t.Fatal(err)
	}
	return w
}

// req sends a request. Headers come as pairs after the body.
func (w *world) req(method, target string, body any, headers ...string) *httptest.ResponseRecorder {
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
	r := httptest.NewRequest(method, target, strings.NewReader(payload))
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, r)
	return rec
}

// as sends a request as the user with this login name, from a session
// named after the user.
func (w *world) as(user, method, target string, body any) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.req(method, target, body, "X-User", user, "X-Token", w.session(user))
}

// session names the session of user in this test. Track writes a session
// once a minute at most, so names must not repeat across tests.
func (w *world) session(user string) string { return "session-" + user + "@" + w.t.Name() }

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

// addUser creates a user through the API and returns it from the store.
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

// passwordLogin runs the real password login of the login system.
func passwordLogin(name, password, code string) error {
	r := httptest.NewRequest("POST", "/api/auth/login?username="+url.QueryEscape(name), nil)
	r.Header.Set("X-Password", url.QueryEscape(password))
	if code != "" {
		r.Header.Set("X-Secret", code)
	}
	_, err := auth.AuthenticatePassword(r, false, false)
	return err
}
