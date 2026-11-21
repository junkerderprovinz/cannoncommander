package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/app"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

const strong = "korrekt-pferd-batterie"

// newServer builds the whole server on a fresh database. before runs once
// the database is there and before the sign-in imports the accounts, as the
// version before would have left them.
func newServer(t *testing.T, before func()) http.Handler {
	t.Helper()
	saved := settings.Config
	savedAssets := assetFs
	t.Cleanup(func() { settings.Config, assetFs = saved, savedAssets })
	if settings.Config.Http.BaseURL == "" {
		settings.Config.Http.BaseURL = "/"
	}
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	settings.Config.Auth.AdminPassword = ""
	src := t.TempDir()
	settings.Config.Server.Sources = []*settings.Source{{Name: "Shares", Path: src}}
	settings.Config.Server.SourceMap = map[string]*settings.Source{src: settings.Config.Server.Sources[0]}
	settings.Config.Server.NameToSource = map[string]*settings.Source{"Shares": settings.Config.Server.Sources[0]}
	settings.InitializeUserResolvers()
	if _, err := state.Initialize(filepath.Join(t.TempDir(), "cc.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	app.MustWireServices(state.Default())
	if before != nil {
		before()
	}
	assetFs = fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	router := http.NewServeMux()
	configureHTTPRouter(router)
	guard, err := mountAPIv1(t.Context(), router)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(accounts.Close)
	return crossOrigin(guard(router))
}

// withAdminPassword gives the admin of the first start a password, as the
// version before kept it.
func withAdminPassword(t *testing.T) func() {
	return func() {
		admin, err := state.GetUserByUsername("admin")
		if err != nil {
			t.Fatal(err)
		}
		if err := state.UpdateUser(&admin, strong, "password"); err != nil {
			t.Fatal(err)
		}
	}
}

func send(t *testing.T, h http.Handler, method, target string, body any, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "https://app.test"+target, bytes.NewReader(raw))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	for i := 0; i+1 < len(header); i += 2 {
		if header[i] == "Cookie" {
			r.Header.Add("Cookie", header[i+1])
			continue
		}
		r.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func signIn(t *testing.T, h http.Handler, name, password string) string {
	t.Helper()
	rec := send(t, h, "POST", "/api/signin/v1/login", map[string]string{"name": name, "password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("sign in: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "__Host-cc-session" {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatal("no session cookie")
	return ""
}

func TestTheSignInKeepsItsContract(t *testing.T) {
	signintest.Run(t, newServer(t, nil), signintest.Options{Password: strong, Name: "admin"})
}

func TestTheSignInKeepsItsContractBelowABasePath(t *testing.T) {
	saved := settings.Config.Http.BaseURL
	t.Cleanup(func() { settings.Config.Http.BaseURL = saved })
	settings.Config.Http.BaseURL = "/files/"
	signintest.Run(t, newServer(t, nil), signintest.Options{Password: strong, Name: "admin", Base: "/files/"})
}

// oldSessionOf issues a session of the version before for u.
func oldSessionOf(t *testing.T, u users.User) string {
	t.Helper()
	now := time.Now()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti": "f00d" + u.Username, "iat": now.Unix(), "exp": now.Add(24 * time.Hour).Unix(), "iss": auth.TokenIssuer, "auth_time": now.Unix(),
	}).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.RegisterSessionToken(tok, u.ID); err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestTheFirstPageSwapsTheOldSessionAndClearsItsCookie(t *testing.T) {
	var old string
	h := newServer(t, func() {
		withAdminPassword(t)()
		admin, _ := state.GetUserByUsername("admin")
		old = oldSessionOf(t, admin)
	})
	page := send(t, h, "GET", "/", nil, "Cookie", accounts.OldCookie+"="+old)
	if page.Code != http.StatusOK {
		t.Fatalf("page: %d", page.Code)
	}
	var fresh string
	cleared := false
	for _, c := range page.Result().Cookies() {
		switch {
		case c.Name == "__Host-cc-session" && c.MaxAge > 0:
			fresh = c.Name + "=" + c.Value
		case c.Name == accounts.OldCookie && c.MaxAge < 0 && c.Domain == "app.test":
			cleared = true
		}
	}
	if fresh == "" || !cleared {
		t.Fatalf("the page did not swap the session and clear the old cookie: %v", page.Result().Cookies())
	}
	if me := send(t, h, "GET", "/api/v1/me", nil, "Cookie", fresh); me.Code != http.StatusOK {
		t.Fatalf("the swapped session does not sign in: %d %s", me.Code, me.Body)
	}
}

func TestNoauthOpensNoSignInAndSaysTheAppIsOpen(t *testing.T) {
	saved := settings.Config.Auth.Methods.NoAuth
	t.Cleanup(func() { settings.Config.Auth.Methods.NoAuth = saved })
	settings.Config.Auth.Methods.NoAuth = true
	h := newServer(t, nil)
	if accounts.SignIn() != nil {
		t.Fatal("noauth opened the sign-in")
	}
	opts := send(t, h, "GET", "/api/signin/v1/options", nil)
	if opts.Code != http.StatusOK || strings.TrimSpace(opts.Body.String()) != `{"open":true}` {
		t.Fatalf("options: %d %s", opts.Code, opts.Body)
	}
	if me := send(t, h, "GET", "/api/v1/me", nil); me.Code != http.StatusOK {
		t.Fatalf("everyone is the admin under noauth: %d %s", me.Code, me.Body)
	}
	if login := send(t, h, "POST", "/api/signin/v1/login", map[string]string{"name": "admin", "password": strong}); login.Code != http.StatusNotFound {
		t.Fatalf("a sign-in under noauth: %d", login.Code)
	}
}

func basic(name, password string) string {
	r, _ := http.NewRequest("GET", "/", nil)
	r.SetBasicAuth(name, password)
	return r.Header.Get("Authorization")
}

func TestWebDAVIgnoresTheSessionCookie(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	session := signIn(t, h, "admin", strong)
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Cookie", session); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a session cookie drove WebDAV: %d", rec.Code)
	}
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", basic("admin", strong)); rec.Code != http.StatusMultiStatus {
		t.Fatalf("the password over Basic: %d %s", rec.Code, rec.Body)
	}
}

func TestAReadTokenOnlyReadsOverWebDAV(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	session := signIn(t, h, "admin", strong)
	made := send(t, h, "POST", "/api/signin/v1/me/tokens", map[string]any{"name": "rclone", "scopes": []string{"read"}}, "Cookie", session)
	var tok struct{ Value string }
	if err := json.Unmarshal(made.Body.Bytes(), &tok); err != nil || made.Code != http.StatusOK {
		t.Fatalf("token: %d %s", made.Code, made.Body)
	}
	creds := basic("wer-auch-immer", tok.Value)
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds); rec.Code != http.StatusMultiStatus {
		t.Fatalf("a read token reads: %d", rec.Code)
	}
	if rec := send(t, h, "MKCOL", "/dav/Shares/neu", nil, "Authorization", creds); rec.Code != http.StatusForbidden {
		t.Fatalf("a read token wrote over WebDAV: %d", rec.Code)
	}
}

func TestANewPasswordEndsWebDAVAtOnce(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	creds := basic("admin", strong)
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds); rec.Code != http.StatusMultiStatus {
		t.Fatalf("before: %d", rec.Code)
	}
	session := signIn(t, h, "admin", strong)
	if rec := send(t, h, "PUT", "/api/signin/v1/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}, "Cookie", session); rec.Code != http.StatusOK {
		t.Fatalf("new password: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the old password still opens WebDAV: %d", rec.Code)
	}
}

func TestTurningOnTwoFactorEndsWebDAVPasswordSignIn(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	creds := basic("admin", strong)
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds); rec.Code != http.StatusMultiStatus {
		t.Fatalf("before: %d", rec.Code)
	}
	session := signIn(t, h, "admin", strong)
	start := send(t, h, "POST", "/api/signin/v1/me/totp", map[string]string{}, "Cookie", session)
	var setup struct{ Secret string }
	if err := json.Unmarshal(start.Body.Bytes(), &setup); err != nil || setup.Secret == "" {
		t.Fatalf("two-factor: %d %s", start.Code, start.Body)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec := send(t, h, "POST", "/api/signin/v1/me/totp/confirm", map[string]string{"code": code}, "Cookie", session); rec.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the password alone still opens WebDAV with two-factor on: %d", rec.Code)
	}
}

func TestBasicCredentialsDoNotOpenTheSignInAPI(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	creds := basic("admin", strong)
	send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", creds)
	for _, route := range []string{"/api/signin/v1/accounts", "/api/signin/v1/log", "/api/v1/me"} {
		if rec := send(t, h, "GET", route, nil, "Authorization", creds); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s with Basic: %d", route, rec.Code)
		}
	}
}

func TestTheTerminalGateFollowsTheSession(t *testing.T) {
	h := newServer(t, withAdminPassword(t))
	session := signIn(t, h, "admin", strong)
	var id string
	gate := terminalGate{}
	accounts.SignIn().Guard(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		id, _ = gate.Session(r)
	})).ServeHTTP(httptest.NewRecorder(), func() *http.Request {
		r := httptest.NewRequest("GET", "https://app.test/api/v1/terminal", nil)
		r.Header.Set("Cookie", session)
		return r
	}())
	if id == "" || !gate.Alive(id) || gate.Open() {
		t.Fatalf("the gate does not see the session: %q", id)
	}
	if rec := send(t, h, "POST", "/api/signin/v1/logout", map[string]string{}, "Cookie", session); rec.Code != http.StatusOK {
		t.Fatalf("sign out: %d", rec.Code)
	}
	if gate.Alive(id) {
		t.Fatal("the terminal outlives its session")
	}
}

func TestWebDAVLeavesTheOldSessionCookieAlone(t *testing.T) {
	var old string
	h := newServer(t, func() {
		withAdminPassword(t)()
		admin, _ := state.GetUserByUsername("admin")
		old = oldSessionOf(t, admin)
	})
	rec := send(t, h, "PROPFIND", "/dav/", nil, "Depth", "1", "Authorization", basic("admin", strong), "Cookie", accounts.OldCookie+"="+old)
	if rec.Code != http.StatusMultiStatus || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("WebDAV took part in the sign-in: %d %v", rec.Code, rec.Result().Cookies())
	}
	if page := send(t, h, "GET", "/", nil, "Cookie", accounts.OldCookie+"="+old); len(page.Result().Cookies()) == 0 {
		t.Fatal("the old session was used up on WebDAV")
	}
}
