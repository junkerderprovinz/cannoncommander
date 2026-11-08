package accounts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func signInWorld(t *testing.T) *world {
	w := newWorld(t)
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	return w
}

func (w *world) signIn(name, password string, remember bool, headers ...string) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.req("POST", "/auth/login", map[string]any{"username": name, "password": password, "remember": remember}, headers...)
}

func (w *world) started() int {
	w.login.mu.Lock()
	defer w.login.mu.Unlock()
	return len(w.login.started)
}

func TestTheSignInPageLearnsTheWaysIn(t *testing.T) {
	w := signInWorld(t)
	got := decode[accounts.SignInOptions](t, w.must(http.StatusOK, w.req("GET", "/auth/options", nil)))
	if !got.Password || !got.Passkey || got.OIDC != nil || got.MinPass < 12 || got.Version == "" {
		t.Fatalf("options = %+v", got)
	}
	settings.Config.Auth.Methods.OidcAuth.Enabled = true
	settings.Config.Frontend.OIDCLoginButtonText = "Mit Authentik anmelden"
	got = decode[accounts.SignInOptions](t, w.must(http.StatusOK, w.req("GET", "/auth/options", nil)))
	if got.OIDC == nil || got.OIDC.Label != "Mit Authentik anmelden" || !strings.HasSuffix(got.OIDC.URL, "api/auth/oidc/login") {
		t.Fatalf("oidc = %+v", got.OIDC)
	}
}

func TestAPasswordSignsInForTheBrowserSession(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	rec := w.must(http.StatusOK, w.signIn(" anna ", strong, false))
	got := decode[accounts.SignIn](t, rec)
	if got.Next != "done" || got.Me == nil || got.Me.Name != "anna" || got.Challenge != "" || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("sign in = %+v", got)
	}
	if w.login.kept[len(w.login.kept)-1] != 0 {
		t.Fatalf("a sign in without remember must end with the browser, got %v", w.login.kept)
	}
}

func TestStayingSignedInFollowsTheSetting(t *testing.T) {
	w := signInWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.signIn("anna", strong, true))
	if got := w.login.kept[len(w.login.kept)-1]; got != 30*24*time.Hour {
		t.Fatalf("without a setting the session lasts %v", got)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO user_prefs (user_id, data, updated_at) VALUES (?, ?, 0)`, strconv.FormatUint(anna.ID, 10), `{"stayDays":7}`); err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusOK, w.signIn("anna", strong, true))
	if got := w.login.kept[len(w.login.kept)-1]; got != 7*24*time.Hour {
		t.Fatalf("with stayDays 7 the session lasts %v", got)
	}
}

func TestAWrongPasswordTellsNoNames(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	wrong := w.must(http.StatusUnauthorized, w.signIn("anna", "falsches-passwort", false))
	unknown := w.must(http.StatusUnauthorized, w.signIn("niemand", "falsches-passwort", false))
	if detail(t, wrong) != detail(t, unknown) {
		t.Fatalf("the answers tell names apart: %q, %q", wrong.Body, unknown.Body)
	}
	w.must(http.StatusBadRequest, w.signIn("anna", "", false))
	if w.started() != 0 {
		t.Fatal("a failed sign in started a session")
	}
}

func TestGuessingPasswordsIsCutShort(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for range 5 {
		w.must(http.StatusUnauthorized, w.signIn("anna", "falsches-passwort", false, "X-IP", "203.0.113.9"))
	}
	rec := w.must(http.StatusTooManyRequests, w.signIn("anna", strong, false, "X-IP", "203.0.113.9"))
	if wait, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || wait < 60 {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	w.must(http.StatusOK, w.signIn("anna", strong, false, "X-IP", "203.0.113.10"))

	for i := range 20 {
		w.must(http.StatusUnauthorized, w.signIn("name"+strconv.Itoa(i), "falsch", false, "X-IP", "198.51.100.7"))
	}
	w.must(http.StatusTooManyRequests, w.signIn("anna", strong, false, "X-IP", "198.51.100.7"))
}

func TestPasswordsCanBeTurnedOff(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	w.must(http.StatusForbidden, w.signIn("anna", strong, false))
}

func TestTwoFactorSignInTakesEachCodeOnce(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	secret, codes := w.turnOnTotp("anna")
	before := w.started()

	step := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, true)))
	if step.Next != "totp" || step.Challenge == "" || step.Me != nil || w.started() != before {
		t.Fatalf("the password alone went through: %+v", step)
	}
	totp := func(challenge, code string) *httptest.ResponseRecorder {
		return w.req("POST", "/auth/totp", map[string]string{"challenge": challenge, "code": code})
	}
	w.must(http.StatusBadRequest, totp(step.Challenge, "000000"))
	// The setup took the code of this time step already.
	w.must(http.StatusBadRequest, totp(step.Challenge, code(t, secret, time.Now())))
	next := code(t, secret, time.Now().Add(30*time.Second))
	done := decode[accounts.SignIn](t, w.must(http.StatusOK, totp(step.Challenge, next)))
	if done.Next != "done" || done.Me.Name != "anna" || w.started() != before+1 || w.login.kept[len(w.login.kept)-1] == 0 {
		t.Fatalf("after the code: %+v", done)
	}
	w.must(http.StatusGone, totp(step.Challenge, next))

	again := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	w.must(http.StatusBadRequest, totp(again.Challenge, next))
	w.must(http.StatusOK, totp(again.Challenge, strings.ToLower(codes[0])))
	third := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	w.must(http.StatusBadRequest, totp(third.Challenge, codes[0]))
	me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil)))
	if me.Recovery != 7 {
		t.Fatalf("recovery codes left: %d", me.Recovery)
	}
}

func TestGuessingSignInCodesIsCutShort(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	secret, _ := w.turnOnTotp("anna")
	step := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	for range 5 {
		w.must(http.StatusBadRequest, w.req("POST", "/auth/totp", map[string]string{"challenge": step.Challenge, "code": "123456"}))
	}
	rec := w.must(http.StatusTooManyRequests, w.req("POST", "/auth/totp", map[string]string{"challenge": step.Challenge, "code": code(t, secret, time.Now().Add(30*time.Second))}))
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
}

func TestAChallengeWorksForItsOwnStepOnly(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.turnOnTotp("anna")
	w.must(http.StatusGone, w.req("POST", "/auth/totp", map[string]string{"challenge": "erfunden", "code": "123456"}))
	step := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	w.must(http.StatusGone, w.req("POST", "/auth/password", map[string]string{"challenge": step.Challenge, "password": "ein-ganz-neues-passwort"}))
	w.must(http.StatusBadRequest, w.req("POST", "/auth/totp", map[string]string{"challenge": step.Challenge}))
}

func TestANewPasswordIsDueAfterTheFirstStart(t *testing.T) {
	w := signInWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	anna.RequirePasswordChange = true
	if err := state.UpdateUser(&anna, "", "requirePasswordChange"); err != nil {
		t.Fatal(err)
	}
	step := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	if step.Next != "password" || w.started() != 0 {
		t.Fatalf("step = %+v", step)
	}
	set := func(pw string) *httptest.ResponseRecorder {
		return w.req("POST", "/auth/password", map[string]string{"challenge": step.Challenge, "password": pw})
	}
	w.must(http.StatusBadRequest, set("kurz"))
	w.must(http.StatusBadRequest, set(strong))
	done := decode[accounts.SignIn](t, w.must(http.StatusOK, set("ein-ganz-neues-passwort")))
	if done.Next != "done" || done.Me.PwChanged == nil {
		t.Fatalf("done = %+v", done)
	}
	w.must(http.StatusUnauthorized, w.signIn("anna", strong, false))
	if got := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", "ein-ganz-neues-passwort", false))); got.Next != "done" {
		t.Fatalf("second sign in = %+v", got)
	}
}

func TestAServerThatAsksForTwoFactorSetsItUpOnTheWayIn(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp = true
	step := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	if step.Next != "totpSetup" || step.Setup == nil || !strings.HasPrefix(step.Setup.QR, "<svg") || w.started() != 0 {
		t.Fatalf("step = %+v", step)
	}
	w.must(http.StatusBadRequest, w.req("POST", "/auth/totp", map[string]string{"challenge": step.Challenge, "code": "000000"}))
	done := decode[accounts.SignIn](t, w.must(http.StatusOK, w.req("POST", "/auth/totp", map[string]string{"challenge": step.Challenge, "code": code(t, step.Setup.Secret, time.Now())})))
	if done.Next != "done" || len(done.Codes) != 8 || !done.Me.Twofa {
		t.Fatalf("done = %+v", done)
	}
	if got := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false))); got.Next != "totp" {
		t.Fatalf("the next sign in = %+v", got)
	}
}

func TestSigningOutEndsThisSession(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	rec := w.must(http.StatusNoContent, w.as("anna", "POST", "/auth/logout", nil))
	if len(w.login.stopped) != 1 || w.login.stopped[0] != w.session("anna") || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("stopped %v", w.login.stopped)
	}
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"password": strong, "name": "skript", "rights": "rw"})))
	w.must(http.StatusForbidden, w.req("POST", "/auth/logout", nil, "X-Api", tok.Secret))
	w.must(http.StatusUnauthorized, w.req("POST", "/auth/logout", nil))
}

// addPasskey registers a passkey for user through the account API.
func (w *world) addPasskey(user string, a *authenticator) {
	w.t.Helper()
	step := decode[struct{ Options json.RawMessage }](w.t, w.must(http.StatusOK, w.as(user, "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop"})))
	w.must(http.StatusCreated, w.as(user, "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop", "credential": a.create(step.Options)}))
}

func (w *world) passkeyChallenge() accounts.PasskeyChallenge {
	w.t.Helper()
	raw := w.must(http.StatusOK, w.req("POST", "/auth/passkey/options", nil)).Body.Bytes()
	var c struct {
		Challenge string
		Options   json.RawMessage
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		w.t.Fatal(err)
	}
	return accounts.PasskeyChallenge{Challenge: c.Challenge, Options: c.Options}
}

func TestAPasskeySignsInWithoutAName(t *testing.T) {
	w := signInWorld(t)
	passkeysAtCannon(t)
	w.addUser("anna", "user", "/mnt/user")
	w.turnOnTotp("anna")
	key := newAuthenticator(t)
	w.addPasskey("anna", key)

	c := w.passkeyChallenge()
	answer := key.get(c.Options.(json.RawMessage))
	done := decode[accounts.SignIn](t, w.must(http.StatusOK, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": answer, "remember": true})))
	if done.Next != "done" || done.Me.Name != "anna" || w.login.kept[len(w.login.kept)-1] == 0 {
		t.Fatalf("done = %+v", done)
	}
	w.must(http.StatusGone, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": answer}))
	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/passkeys", nil)))
	if len(list.Items) != 1 || list.Items[0].Last == nil {
		t.Fatalf("the passkey does not show its use: %+v", list)
	}
}

func TestAPasskeyFromElsewhereDoesNotSignIn(t *testing.T) {
	w := signInWorld(t)
	passkeysAtCannon(t)
	w.addUser("anna", "user", "/mnt/user")
	key := newAuthenticator(t)
	w.addPasskey("anna", key)
	before := w.started()

	key.origin = "https://cannon.example.org"
	c := w.passkeyChallenge()
	w.must(http.StatusUnauthorized, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": key.get(c.Options.(json.RawMessage))}))

	key.origin = passkeyOrigin
	stranger := newAuthenticator(t)
	stranger.user = key.user
	c = w.passkeyChallenge()
	w.must(http.StatusUnauthorized, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": stranger.get(c.Options.(json.RawMessage))}))
	if w.started() != before {
		t.Fatal("a foreign passkey started a session")
	}
}

func TestPasskeySignInNeedsAHostNameAndPasswords(t *testing.T) {
	w := signInWorld(t)
	answer := map[string]any{"challenge": "x", "credential": map[string]any{}}
	w.login.onIP = true
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey/options", nil))
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey", answer))
	w.login.onIP = false
	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey/options", nil))
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey", answer))
}
