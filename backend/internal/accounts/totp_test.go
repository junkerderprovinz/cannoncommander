package accounts_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

func code(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := totp.GenerateCode(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// turnOnTotp runs the setup for user and returns the secret and the
// recovery codes.
func (w *world) turnOnTotp(user string) (string, []string) {
	w.t.Helper()
	setup := decode[accounts.TotpSetup](w.t, w.must(http.StatusOK, w.as(user, "POST", "/me/totp", map[string]any{})))
	codes := decode[accounts.RecoveryList](w.t, w.must(http.StatusOK, w.as(user, "POST", "/me/totp", map[string]any{"code": code(w.t, setup.Secret, time.Now())})))
	return setup.Secret, codes.Codes
}

func TestTurningOnTwoFactorLogin(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	setup := decode[accounts.TotpSetup](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{})))
	if len(setup.Secret) < 16 || !strings.HasPrefix(setup.URI, "otpauth://totp/") || !strings.Contains(setup.URI, setup.Secret) || !strings.HasPrefix(setup.QR, "<svg") || strings.Count(setup.QR, "<rect") < 100 {
		t.Fatalf("setup = %+v", setup)
	}
	if u, _ := state.GetUserByUsername("anna"); u.TOTPSecret != "" {
		t.Fatal("two factor login is on before the first code")
	}
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/totp", map[string]any{"code": "000000"}))
	got := decode[accounts.RecoveryList](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{"code": code(t, setup.Secret, time.Now())})))
	if len(got.Codes) != 8 || len(got.Codes[0]) != 9 || got.Codes[0][4] != '-' {
		t.Fatalf("codes = %v", got.Codes)
	}
	me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil)))
	if !me.Twofa || me.Recovery != 8 {
		t.Fatalf("me = %+v", me)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/totp", map[string]any{}))

	if err := w.passwordLogin("anna", strong, ""); err == nil {
		t.Fatal("the login takes the password alone")
	}
	if err := w.passwordLogin("anna", strong, code(t, setup.Secret, time.Now().Add(30*time.Second))); err != nil {
		t.Fatalf("login with a code: %v", err)
	}
}

func TestTheSetupWantsTheCodeOfItsOwnSecret(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/totp", map[string]any{"code": "123456"}))
	first := decode[accounts.TotpSetup](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{})))
	second := decode[accounts.TotpSetup](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{})))
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/totp", map[string]any{"code": code(t, first.Secret, time.Now())}))
	w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{"code": code(t, second.Secret, time.Now())}))
}

func TestGuessingCodesIsCutShort(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	setup := decode[accounts.TotpSetup](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{})))
	for range 5 {
		w.as("anna", "POST", "/me/totp", map[string]any{"code": "000000"})
	}
	w.must(http.StatusTooManyRequests, w.as("anna", "POST", "/me/totp", map[string]any{"code": code(t, setup.Secret, time.Now())}))
}

func TestACodeCountsOnce(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	secret, _ := w.turnOnTotp("anna")
	used := code(t, secret, time.Now())
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/totp", map[string]any{"code": used, "regenerate": true}))
	next := code(t, secret, time.Now().Add(30*time.Second))
	w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{"code": next, "regenerate": true}))
}

func TestRecoveryCodesStandInForTheApp(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	_, codes := w.turnOnTotp("anna")

	if err := w.passwordLogin("anna", strong, strings.ToLower(codes[0])); err != nil {
		t.Fatalf("login with a recovery code: %v", err)
	}
	if err := w.passwordLogin("anna", strong, codes[0]); err == nil {
		t.Fatal("a recovery code logged in twice")
	}
	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil))); me.Recovery != 7 {
		t.Fatalf("left = %d", me.Recovery)
	}

	fresh := decode[accounts.RecoveryList](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/totp", map[string]any{"code": codes[1], "regenerate": true}))).Codes
	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil))); me.Recovery != 8 {
		t.Fatalf("left after new codes = %d", me.Recovery)
	}
	if err := w.passwordLogin("anna", strong, codes[2]); err == nil {
		t.Fatal("an old recovery code still works")
	}

	w.must(http.StatusBadRequest, w.as("anna", "DELETE", "/me/totp", map[string]any{"code": "AAAA-AAAA"}))
	w.must(http.StatusNoContent, w.as("anna", "DELETE", "/me/totp", map[string]any{"code": fresh[0]}))
	me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil)))
	if me.Twofa || me.Recovery != 0 {
		t.Fatalf("me = %+v", me)
	}
	if err := w.passwordLogin("anna", strong, ""); err != nil {
		t.Fatalf("the password alone logs in again: %v", err)
	}
	w.must(http.StatusConflict, w.as("anna", "DELETE", "/me/totp", map[string]any{"code": fresh[1]}))
}
