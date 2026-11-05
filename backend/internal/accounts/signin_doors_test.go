package accounts_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestOneClientBehindAnUnnamedProxyDoesNotLockOutEveryone(t *testing.T) {
	w := signInWorld(t)
	turnOnPasskeys(t)
	w.addUser("anna", "user", "/mnt/user")
	viaProxy := []string{"X-IP", "172.18.0.2", "X-Forwarded-For", "203.0.113.9"}
	for i := range 25 {
		w.must(http.StatusUnauthorized, w.signIn("name"+strconv.Itoa(i), "falsch", false, viaProxy...))
	}
	w.must(http.StatusOK, w.signIn("anna", strong, false, "X-IP", "172.18.0.2", "X-Forwarded-For", "203.0.113.10"))
	w.must(http.StatusOK, w.req("POST", "/auth/passkey/options", nil, viaProxy...))
}

func TestBadResetCodesBehindAnUnnamedProxyDoNotCloseTheResetForEveryone(t *testing.T) {
	w := newWorld(t)
	log := catchCodes(t)
	w.addUser("anna", "user", "/mnt/user")
	w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	viaProxy := []string{"X-IP", "172.18.0.2", "X-Forwarded-For", "203.0.113.9"}
	for range 6 {
		w.must(http.StatusBadRequest, w.req("POST", "/auth/reset", map[string]string{"code": "AAAA-AAAA-AAAA", "password": "ein-ganz-neues-passwort"}, viaProxy...))
	}
	w.must(http.StatusOK, w.req("POST", "/auth/reset", map[string]string{"code": log.last("anna"), "password": "ein-ganz-neues-passwort"}, viaProxy...))
}

func TestGuessingFromAcrossAnIPv6NetworkIsCutShort(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for i := range 5 {
		w.must(http.StatusUnauthorized, w.signIn("anna", "falsches-passwort", false, "X-IP", "2001:db8:1:2::"+strconv.Itoa(i+1)))
	}
	w.must(http.StatusTooManyRequests, w.signIn("anna", strong, false, "X-IP", "2001:db8:1:2::99"))
}

func TestResettingThePasswordRemovesPasskeys(t *testing.T) {
	w := signInWorld(t)
	turnOnPasskeys(t)
	log := catchCodes(t)
	w.addUser("anna", "user", "/mnt/user")
	key := newAuthenticator(t)
	w.addPasskey("anna", key)
	w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	w.must(http.StatusOK, w.req("POST", "/auth/reset", map[string]string{"code": log.last("anna"), "password": "ein-ganz-neues-passwort"}))

	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/passkeys", nil)))
	if len(list.Items) != 0 {
		t.Errorf("%d passkeys left", len(list.Items))
	}
	c := w.passkeyChallenge()
	w.must(http.StatusUnauthorized, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": key.get(c.Options.(json.RawMessage))}))
}

func TestAPendingTwoFactorSetupEndsWhenTheOwnerSetsItUp(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp = true
	stale := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	owner := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	w.must(http.StatusOK, w.req("POST", "/auth/totp", map[string]string{"challenge": owner.Challenge, "code": code(t, owner.Setup.Secret, time.Now())}))

	before, err := state.GetUserByUsername("anna")
	if err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusGone, w.req("POST", "/auth/totp", map[string]string{"challenge": stale.Challenge, "code": code(t, stale.Setup.Secret, time.Now().Add(30*time.Second))}))
	after, err := state.GetUserByUsername("anna")
	if err != nil {
		t.Fatal(err)
	}
	if after.TOTPSecret != before.TOTPSecret {
		t.Error("the old sign in replaced the secret")
	}
}

func TestAPendingSignInEndsWhenThePasswordChanges(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	secret, _ := w.turnOnTotp("anna")
	stale := decode[accounts.SignIn](t, w.must(http.StatusOK, w.signIn("anna", strong, false)))
	w.must(http.StatusOK, w.as("anna", "PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))
	w.must(http.StatusGone, w.req("POST", "/auth/totp", map[string]string{"challenge": stale.Challenge, "code": code(t, secret, time.Now().Add(30*time.Second))}))
}

func TestATokenCannotEndASession(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw"})))
	w.must(http.StatusForbidden, w.req("DELETE", "/me/sessions/irgendeine", nil, "X-Api", tok.Secret))
}

func TestAnAccountFromElsewhereGetsNoPasskey(t *testing.T) {
	w := signInWorld(t)
	turnOnPasskeys(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	key := newAuthenticator(t)
	w.addPasskey("anna", key)

	anna.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&anna, "", "LoginMethod"); err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Zweiter"}))
	c := w.passkeyChallenge()
	w.must(http.StatusUnauthorized, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": key.get(c.Options.(json.RawMessage))}))
	if w.started() != 0 {
		t.Error("a session was started")
	}
}

func TestAPasskeyDoesNotWipeTheCountOfAnAddress(t *testing.T) {
	w := signInWorld(t)
	turnOnPasskeys(t)
	w.addUser("anna", "user", "/mnt/user")
	key := newAuthenticator(t)
	w.addPasskey("anna", key)
	for i := range 19 {
		w.must(http.StatusUnauthorized, w.signIn("name"+strconv.Itoa(i), "falsch", false))
	}
	c := w.passkeyChallenge()
	w.must(http.StatusOK, w.req("POST", "/auth/passkey", map[string]any{"challenge": c.Challenge, "credential": key.get(c.Options.(json.RawMessage))}))
	w.must(http.StatusUnauthorized, w.signIn("noch-ein-name", "falsch", false))
	w.must(http.StatusTooManyRequests, w.signIn("anna", strong, false))
}
