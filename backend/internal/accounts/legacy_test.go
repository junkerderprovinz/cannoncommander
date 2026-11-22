package accounts_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

// oldSession issues a session of the version before for u that signed in
// at signedIn.
func (w *world) oldSession(u users.User, signedIn time.Time) string {
	w.t.Helper()
	now := time.Now()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti": u.Username + strconv.FormatInt(now.UnixNano(), 16), "iat": now.Unix(), "exp": now.Add(24 * time.Hour).Unix(),
		"iss": auth.TokenIssuer, "auth_time": signedIn.Unix(), "stay": true,
	}).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		w.t.Fatal(err)
	}
	if err := state.RegisterSessionToken(tok, u.ID); err != nil {
		w.t.Fatal(err)
	}
	return tok
}

func oldCookie(raw string) header { return header{"Cookie", accounts.OldCookie + "=" + raw} }

func TestAnOldSessionSignsInAndIsSwappedForANewOne(t *testing.T) {
	w := newWorld(t)
	old := w.oldSession(w.admin, time.Now())
	w.open()
	rec := w.must(http.StatusOK, w.req("GET", "/me", nil, oldCookie(old)))
	if decode[accounts.Me](t, rec).Name != "jdp" {
		t.Fatalf("signed in as %s", rec.Body)
	}
	fresh := sessionCookie(t, rec)
	w.must(http.StatusOK, w.req("GET", "/me", nil, header{"Cookie", fresh}))
}

func TestOldCookiesOfTwoAccountsSwapNeither(t *testing.T) {
	w := newWorld(t)
	anna := w.oldUser("anna", strong, false)
	mine, planted := w.oldSession(w.admin, time.Now()), w.oldSession(anna, time.Now())
	w.open()
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, oldCookie(mine), oldCookie(planted)))
}

func TestAnOldSessionCannotMakeATokenWithoutThePassword(t *testing.T) {
	w := newWorld(t)
	old := w.oldSession(w.admin, time.Now().Add(-time.Hour))
	w.open()
	fresh := sessionCookie(t, w.must(http.StatusOK, w.req("GET", "/me", nil, oldCookie(old))))
	rec := w.req("POST", "/api/signin/v1/me/tokens", map[string]any{"name": "skript", "scopes": []string{"all"}}, header{"Cookie", fresh})
	if rec.Code != http.StatusForbidden || code(t, rec) != "signInAgain" {
		t.Fatalf("a renewed old session made a token: %d %s", rec.Code, rec.Body)
	}
}

func TestAnOldSessionIsNoToken(t *testing.T) {
	w := newWorld(t)
	old := w.oldSession(w.admin, time.Now())
	w.open()
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, bearer(old)))
}

func TestDropLeavesTheOldPasswordUnusableButNeverEmpty(t *testing.T) {
	w := newWorld(t)
	token := w.oldToken(&w.admin, "skript", users.Permissions{}, true, false)
	session := w.oldSession(w.admin, time.Now())
	w.withSecondFactor(&w.admin, "ABCD2345")
	w.admin.PasskeyCredentials = []users.WebAuthnCredential{{ID: "bGFwdG9w", PublicKey: "a2V5", Name: "Laptop"}}
	if err := state.UpdateUser(&w.admin, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}
	w.open()

	b := w.signInWithCode("jdp", "ABCD2345")
	w.must(http.StatusOK, b.do("PUT", "/api/signin/v1/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))

	u, err := state.GetUserByID(w.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Password == "" || utils.CheckPwd(strong, u.Password) == nil || utils.CheckPwd("ein-ganz-neues-passwort", u.Password) == nil {
		t.Fatal("the version before can still use a password, or has none and opens")
	}
	if u.TOTPSecret != "" || len(u.PasskeyCredentials) != 0 || len(u.Tokens) != 0 {
		t.Fatalf("old ways in are left: totp %q, passkeys %d, tokens %d", u.TOTPSecret, len(u.PasskeyCredentials), len(u.Tokens))
	}
	if !state.IsTokenRevoked(token) || !state.IsTokenRevoked(session) {
		t.Fatal("the old token or session is not revoked in the old registry")
	}
	if _, _, ok := state.HashedTokenOwner(token); ok {
		t.Fatal("the old registry still knows the token")
	}
	var codes int
	if err := w.deps.DB.QueryRow(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ?`, strconv.FormatUint(u.ID, 10)).Scan(&codes); err != nil || codes != 0 {
		t.Fatalf("recovery codes left: %d, %v", codes, err)
	}
}

func TestABlockedOIDCAccountCannotComeBackThroughTheOldVersion(t *testing.T) {
	w := newWorld(t)
	bert := w.oldUser("bert", "", false)
	bert.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&bert, "", "loginMethod"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO oidc_identities (issuer, subject, user_id) VALUES ('https://idp.test', 's-1', ?)`, strconv.FormatUint(bert.ID, 10)); err != nil {
		t.Fatal(err)
	}
	w.open()
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/bert", map[string]any{"blocked": true}))

	u, _ := state.GetUserByID(bert.ID)
	if u.LoginMethod != users.LoginMethodPassword || u.Password == "" {
		t.Fatalf("the old version would still bind bert by name: %s %q", u.LoginMethod, u.Password)
	}
	var bound int
	if err := w.deps.DB.QueryRow(`SELECT COUNT(*) FROM oidc_identities WHERE user_id = ?`, strconv.FormatUint(bert.ID, 10)).Scan(&bound); err != nil || bound != 0 {
		t.Fatalf("the binding of a blocked account stays: %d, %v", bound, err)
	}
}

func TestAfterTheFirstPasswordTheOldVersionStaysClosed(t *testing.T) {
	w := newBareWorld(t)
	admin := w.oldUser(settings.PasswordAdminUsername(), "", true)
	w.open()
	w.must(http.StatusOK, w.browser().do("PUT", "/api/signin/v1/me/password", map[string]string{"next": strong}))
	if state.OpenAccess() {
		t.Fatal("open after the first password")
	}
	u, _ := state.GetUserByID(admin.ID)
	if u.Password == "" || utils.CheckPwd(strong, u.Password) == nil {
		t.Fatal("the version before would be open, or let the password in that it never saw")
	}
}

func TestAccountsMadeInTheNewVersionSignInNowhereInTheOldOne(t *testing.T) {
	w := newWorld(t).open()
	bert := w.addUser("bert", "user", "/mnt/user")
	if bert.Password == "" || utils.CheckPwd(strong, bert.Password) == nil || bert.LoginMethod != users.LoginMethodPassword {
		t.Fatalf("the row of a new account: %s %q", bert.LoginMethod, bert.Password)
	}
}

func TestARestartAfterOwnChangesStartsAfterARollbackThatChangedNothing(t *testing.T) {
	w := newWorld(t)
	w.oldToken(&w.admin, "skript", users.Permissions{}, true, false)
	w.open()
	w.addUser("bert", "user", "/mnt/user")
	b := w.browser()
	w.must(http.StatusOK, b.do("POST", "/api/signin/v1/login", map[string]string{"name": "jdp", "password": strong}))
	w.must(http.StatusOK, b.do("PUT", "/api/signin/v1/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))
	accounts.Close()
	if _, err := w.deps.DB.Exec(`UPDATE schema_version SET version = version - 1`); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Initialize(filepath.Join(w.base, "state.db")); err != nil {
		t.Fatalf("own changes look like the older version's: %v", err)
	}
}

func TestAReimportStartsFromWhatTheOlderVersionLeft(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("bert", "user", "/mnt/user")
	accounts.Close()
	// The older version runs, sets a password for jdp and starts again.
	hash, err := utils.HashPwd("im-alten-gesetzt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`UPDATE users SET user_data = json_set(user_data, '$.password', ?) WHERE user_id = ?`, hash, strconv.FormatUint(w.admin.ID, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`UPDATE schema_version SET version = version - 1`); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.base, "state.db")
	if _, err := state.Initialize(path); err == nil {
		t.Fatal("started on what the older version changed")
	}
	t.Setenv("CC_SIGNIN_REIMPORT", "1")
	if _, err := state.Initialize(path); err != nil {
		t.Fatal(err)
	}
	revoked := 0
	accounts.OnReimport(func(context.Context, *sql.DB) (int, error) { revoked++; return 0, nil })
	t.Cleanup(func() { accounts.OnReimport(func(context.Context, *sql.DB) (int, error) { return 0, nil }) })
	w.deps.DB = state.MainDB()
	w.open()
	if revoked != 1 {
		t.Fatalf("the reimport revoked MCP keys and grants %d times", revoked)
	}
	if _, err := w.signIn("jdp", "im-alten-gesetzt"); err != nil {
		t.Fatalf("the password the older version set does not sign in: %v", err)
	}
	if _, err := w.signIn("jdp", strong); err == nil {
		t.Fatal("the password from before the rollback still signs in")
	}
}

// oldRow is the row of name and how many OIDC bindings the version before
// would sign it in with.
func (w *world) oldRow(name string) (users.User, int) {
	w.t.Helper()
	u, err := state.GetUserByUsername(name)
	if err != nil {
		w.t.Fatal(err)
	}
	var bound int
	if err := w.deps.DB.QueryRow(`SELECT COUNT(*) FROM oidc_identities WHERE user_id = ?`, strconv.FormatUint(u.ID, 10)).Scan(&bound); err != nil {
		w.t.Fatal(err)
	}
	return u, bound
}

func TestBlockingANewcomerFromTheConfigShutsItOutOfTheOldVersion(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	w.open()
	w.throughProvider(p, signintest.Person{Subject: "s-9", Name: "erna"})
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/erna", map[string]any{"blocked": true}))
	if u, bound := w.oldRow("erna"); u.LoginMethod != users.LoginMethodPassword || u.Password == "" || bound != 0 {
		t.Fatalf("the version before would let blocked erna in: %s, %d bindings", u.LoginMethod, bound)
	}
}

func TestBlockingAnOIDCAccountAfterAnEarlierEndingShutsItOutOfTheOldVersion(t *testing.T) {
	w := newWorld(t)
	bert := w.oldUser("bert", "", false)
	bert.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&bert, "", "loginMethod"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO oidc_identities (issuer, subject, user_id) VALUES ('https://idp.test', 's-1', ?)`, strconv.FormatUint(bert.ID, 10)); err != nil {
		t.Fatal(err)
	}
	w.open()
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/bert", map[string]any{"revoke": true}))
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/bert", map[string]any{"blocked": true}))
	if u, bound := w.oldRow("bert"); u.LoginMethod != users.LoginMethodPassword || u.Password == "" || bound != 0 {
		t.Fatalf("the version before would let blocked bert in: %s, %d bindings", u.LoginMethod, bound)
	}
}

// failRemoval makes the removal of the account with this id fail in a hook
// of another package, as a share that cannot be removed would.
var failRemoval atomic.Uint64

func init() {
	accounts.OnDelete(func(_ context.Context, _ v1.Deps, id uint64) error {
		if failRemoval.Load() == id {
			return errors.New("the share is busy")
		}
		return nil
	})
}

func TestARemovalThatKeepsFailingLeavesNoOldRowThatSignsIn(t *testing.T) {
	w := newWorld(t)
	anna := w.oldUser("anna", strong, false)
	token := w.oldToken(&anna, "skript", users.Permissions{}, true, false)
	w.open()
	failRemoval.Store(anna.ID)
	t.Cleanup(func() { failRemoval.Store(0) })
	w.must(http.StatusNoContent, w.as("jdp", "DELETE", "/admin/users/anna", nil))

	u, err := state.GetUserByID(anna.ID)
	if err != nil {
		t.Fatal("the row went before the hooks were done")
	}
	if u.Password == "" || utils.CheckPwd(strong, u.Password) == nil || !state.IsTokenRevoked(token) {
		t.Fatal("while the removal waits, the version before still lets anna in")
	}
	failRemoval.Store(0)
	if err := accounts.Ended(w.deps)(t.Context(), strconv.FormatUint(anna.ID, 10), signin.EndRemoved); err != nil {
		t.Fatal(err)
	}
	if _, err := state.GetUserByID(anna.ID); err == nil {
		t.Fatal("the row is still there once the hooks took the removal")
	}
}

func TestWhatTheOlderVersionRetiredStaysOut(t *testing.T) {
	w := newWorld(t)
	session := w.oldSession(w.admin, time.Now())
	token := w.oldToken(&w.admin, "skript", users.Permissions{}, true, false)
	for _, raw := range []string{session, token} {
		if _, err := w.deps.DB.Exec(`INSERT INTO revoked_tokens (token_hash, revoked_at) VALUES (?, ?)`, utils.HashSHA256(raw), time.Now().Add(-time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Initialize(filepath.Join(w.base, "state.db")); err != nil {
		t.Fatal(err)
	}
	w.deps.DB = state.MainDB()
	w.open()
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, oldCookie(session)))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, bearer(token)))
}

func TestDroppingARemovedOIDCAccountShutsItOutOfTheOldVersion(t *testing.T) {
	w := newWorld(t)
	bert := w.oldUser("bert", "", false)
	bert.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&bert, "", "loginMethod"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO oidc_identities (issuer, subject, user_id) VALUES ('https://idp.test', 's-1', ?)`, strconv.FormatUint(bert.ID, 10)); err != nil {
		t.Fatal(err)
	}
	w.open()
	if err := accounts.Drop(w.deps)(t.Context(), strconv.FormatUint(bert.ID, 10), signin.EndRemoved); err != nil {
		t.Fatal(err)
	}
	if u, bound := w.oldRow("bert"); u.LoginMethod != users.LoginMethodPassword || u.Password == "" || bound != 0 {
		t.Fatalf("the version before would let removed bert in: %s, %d bindings", u.LoginMethod, bound)
	}
}
