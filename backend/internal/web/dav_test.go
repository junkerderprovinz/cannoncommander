package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func davEnv(t *testing.T) *users.User {
	t.Helper()
	setupTestEnv(t)
	saved := settings.Config
	settings.Config.Auth.Key = "key"
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	t.Cleanup(func() { settings.Config = saved })
	accounts.UseLogin(sessionLogin{})
	return createTokenAuthUser(t, "anna", users.Permissions{Api: true})
}

func davRequest(name, password string) *http.Request {
	r := httptest.NewRequest("PROPFIND", "/dav/", http.NoBody)
	r.SetBasicAuth(name, password)
	return r
}

func TestDavTakesAnAPITokenAsThePassword(t *testing.T) {
	u := davEnv(t)
	raw := registerApiToken(t, u, "rclone", users.Permissions{}, true, nil)
	rec := httptest.NewRecorder()
	id, err := davSignIn(rec, davRequest("wer-auch-immer", raw))
	if err != nil || id.User.Username != "anna" || id.ReadOnly {
		t.Fatalf("got %+v, %v", id, err)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("a token login must not plant a cookie")
	}
	var seen int
	if err := state.MainDB().QueryRow(`SELECT COUNT(*) FROM token_use WHERE token_hash = ?`, utils.HashSHA256(raw)).Scan(&seen); err != nil || seen != 1 {
		t.Fatalf("the token list does not know the token was used: %d, %v", seen, err)
	}
}

func TestDavMarksAReadOnlyTokenAndTakesItsRights(t *testing.T) {
	u := davEnv(t)
	raw := registerApiToken(t, u, "lesen", users.Permissions{}, true, func(tk *users.AuthToken) { tk.ReadOnly = true })
	id, err := davSignIn(httptest.NewRecorder(), davRequest("anna", raw))
	if err != nil || !id.ReadOnly {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestDavDoesNotTakeASessionTokenAsThePassword(t *testing.T) {
	u := davEnv(t)
	if _, err := davSignIn(httptest.NewRecorder(), davRequest("anna", testSessionToken(t, u, 0))); err == nil {
		t.Fatal("a session token signed in")
	}
}

func TestDavIgnoresTheCookieTheQueryAndBearer(t *testing.T) {
	u := davEnv(t)
	session := testSessionToken(t, u, 0)
	raw := registerApiToken(t, u, "rclone", users.Permissions{}, true, nil)

	cookie := httptest.NewRequest("PROPFIND", "/dav/", http.NoBody)
	cookie.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	query := httptest.NewRequest("PROPFIND", "/dav/?auth="+raw, http.NoBody)
	bearer := httptest.NewRequest("PROPFIND", "/dav/", http.NoBody)
	bearer.Header.Set("Authorization", "Bearer "+raw)
	for _, r := range []*http.Request{cookie, query, bearer} {
		if _, err := davSignIn(httptest.NewRecorder(), r); err == nil {
			t.Fatalf("signed in with %v", r.Header)
		}
	}
}

func TestDavRefusesARevokedToken(t *testing.T) {
	u := davEnv(t)
	raw := registerApiToken(t, u, "rclone", users.Permissions{}, true, nil)
	if err := state.RevokeToken(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := davSignIn(httptest.NewRecorder(), davRequest("anna", raw)); err == nil {
		t.Fatal("a revoked token signed in")
	}
}

func TestDavCountsABadTokenAsAFailedTry(t *testing.T) {
	davEnv(t)
	var wait *accounts.TooManyTries
	for range 5 {
		if _, err := davSignIn(httptest.NewRecorder(), davRequest("anna", "kein.gueltiges.token")); err == nil || errors.As(err, &wait) {
			t.Fatalf("too early: %v", err)
		}
	}
	if _, err := davSignIn(httptest.NewRecorder(), davRequest("anna", "kein.gueltiges.token")); !errors.As(err, &wait) {
		t.Fatalf("no lockout: %v", err)
	}
}

func TestDavKeepsAValidTokenWorkingDuringALockout(t *testing.T) {
	u := davEnv(t)
	raw := registerApiToken(t, u, "rclone", users.Permissions{}, true, nil)
	for range 5 {
		_, _ = davSignIn(httptest.NewRecorder(), davRequest("anna", "falsch"))
	}
	if _, err := davSignIn(httptest.NewRecorder(), davRequest("anna", raw)); err != nil {
		t.Fatalf("somebody guessing a password must not stop the token: %v", err)
	}
}

func TestDavTakesAPassword(t *testing.T) {
	davEnv(t)
	u := &users.User{FrontendUser: users.FrontendUser{Username: "bea", LoginMethod: users.LoginMethodPassword}}
	if err := state.CreateUser(u, "korrekt-pferd-batterie"); err != nil {
		t.Fatal(err)
	}
	id, err := davSignIn(httptest.NewRecorder(), davRequest("bea", "korrekt-pferd-batterie"))
	if err != nil || id.User.Username != "bea" || id.ReadOnly {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestDavAnswersAsTheOpenUserWithoutSignIn(t *testing.T) {
	davEnv(t)
	settings.Config.Auth.Methods.NoAuth = true
	admin := &users.User{FrontendUser: users.FrontendUser{Username: "admin", LoginMethod: users.LoginMethodPassword, Permissions: users.Permissions{Admin: true}}}
	if err := state.CreateUser(admin, "korrekt-pferd-batterie"); err != nil {
		t.Fatal(err)
	}
	id, err := davSignIn(httptest.NewRecorder(), httptest.NewRequest("PROPFIND", "/dav/", http.NoBody))
	if err != nil || id.User == nil {
		t.Fatalf("got %+v, %v", id, err)
	}
}

func TestDavIsMountedAndFollowsTheSwitch(t *testing.T) {
	davEnv(t)
	settings.Config.Http.BaseURL = "/cc/"
	mux := http.NewServeMux()
	mountDAV(mux, v1.Deps{Hub: hub.New(hub.Options{}), DB: state.MainDB()})
	ask := func(p string) int {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("OPTIONS", p, http.NoBody))
		return rec.Code
	}
	settings.Config.Http.DisableWebDAV = true
	for _, p := range []string{"/cc/dav", "/cc/dav/", "/cc/dav/mnt/user"} {
		if got := ask(p); got != http.StatusNotFound {
			t.Fatalf("off, %s: status %d", p, got)
		}
	}
	settings.Config.Http.DisableWebDAV = false
	for _, p := range []string{"/cc/dav", "/cc/dav/", "/cc/dav/mnt/user"} {
		if got := ask(p); got != http.StatusUnauthorized {
			t.Fatalf("on, %s: status %d", p, got)
		}
	}
}

func TestAFinishedPartFileKeepsTheLabelsOfTheFileItReplaces(t *testing.T) {
	db := testutil.MainDB(t)
	if _, err := db.Exec(`INSERT INTO label_items (path, dir, colour, updated_at) VALUES ('/mnt/user/a.jpg', '/mnt/user', 'red', 0), ('/mnt/user/b.jpg', '/mnt/user', 'blue', 0)`); err != nil {
		t.Fatal(err)
	}
	m := labelMover{v1.Deps{DB: db, Hub: hub.New(hub.Options{})}}
	m.Moved("/mnt/user/.a.jpg.1234.cc-part", "/mnt/user/a.jpg")
	m.Moved("/mnt/user/b.jpg", "/mnt/user/c.jpg")
	colour := func(p string) string {
		var c string
		_ = db.QueryRow(`SELECT colour FROM label_items WHERE path = ?`, p).Scan(&c)
		return c
	}
	if colour("/mnt/user/a.jpg") != "red" {
		t.Fatal("the rename of a part file dropped the label")
	}
	if colour("/mnt/user/c.jpg") != "blue" {
		t.Fatal("an ordinary rename has to carry the label")
	}
}

func TestDavLetsEveryoneInWhileNoAccountHasAPassword(t *testing.T) {
	davEnv(t)
	if err := state.DeleteUserByUsername("keeper"); err != nil {
		t.Fatal(err)
	}
	admin := &users.User{FrontendUser: users.FrontendUser{Username: "admin", LoginMethod: users.LoginMethodPassword, Permissions: users.Permissions{Admin: true}}}
	if err := state.CreateUser(admin, ""); err != nil {
		t.Fatal(err)
	}
	id, err := davSignIn(httptest.NewRecorder(), httptest.NewRequest("PROPFIND", "/dav/", http.NoBody))
	if err != nil || id.User == nil || id.User.Username != "admin" {
		t.Fatalf("got %+v, %v", id, err)
	}
}
