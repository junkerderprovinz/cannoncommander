package accounts_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func (w *world) basic(name, password string) (*users.User, error) {
	return accounts.BasicLogin(w.deps.DB, httptest.NewRequest("PROPFIND", "/dav/", nil), name, password)
}

func TestBasicLoginTakesNameAndPassword(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	u, err := w.basic("anna", strong)
	if err != nil || u.Username != "anna" {
		t.Fatalf("got %v, %v", u, err)
	}
	if u, err := w.basic("anna", strong); err != nil || u.Username != "anna" {
		t.Fatalf("second request: %v, %v", u, err)
	}
	if _, err := w.basic("anna", "falsch"); err == nil {
		t.Fatal("a wrong password passed")
	}
	if _, err := w.basic("niemand", strong); err == nil {
		t.Fatal("an unknown name passed")
	}
}

func TestBasicLoginStopsAfterAChangedPassword(t *testing.T) {
	w := signInWorld(t)
	u := w.addUser("anna", "user", "/mnt/user")
	if _, err := w.basic("anna", strong); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(&u, "ein-ganz-anderes-passwort", "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("the old password still works")
	}
	if _, err := w.basic("anna", "ein-ganz-anderes-passwort"); err != nil {
		t.Fatal(err)
	}
}

func TestBasicLoginRefusesAnAccountThatNeedsASecondFactor(t *testing.T) {
	w := signInWorld(t)
	u := w.addUser("anna", "user", "/mnt/user")
	if _, err := w.basic("anna", strong); err != nil {
		t.Fatal(err)
	}
	u.TOTPSecret = "geheim"
	if err := state.UpdateUser(&u, "", "totpSecret"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("a password alone passed for an account with TOTP")
	}
}

func TestBasicLoginRefusesWhereTwoFactorIsEnforced(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp = true
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("a password alone passed although the server enforces TOTP")
	}
}

func TestBasicLoginRefusesWhatTheSignInPageWouldNotFinish(t *testing.T) {
	w := signInWorld(t)
	u := w.addUser("anna", "user", "/mnt/user")
	u.RequirePasswordChange = true
	if err := state.UpdateUser(&u, "", "requirePasswordChange"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("a password that has to be changed passed")
	}
	u.RequirePasswordChange = false
	u.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&u, "", "requirePasswordChange", "loginMethod"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("the password of a single sign on account passed")
	}
}

func TestBasicLoginRefusesWherePasswordsAreOff(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	if _, err := w.basic("anna", strong); err == nil {
		t.Fatal("a password passed on a server that takes none")
	}
}

func TestBasicLoginLocksOutAfterFiveWrongPasswords(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for range 5 {
		if _, err := w.basic("anna", "falsch"); err == nil {
			t.Fatal("a wrong password passed")
		}
	}
	_, err := w.basic("anna", strong)
	var wait *accounts.TooManyTries
	if !errors.As(err, &wait) || wait.Wait <= 0 {
		t.Fatalf("the right password after five wrong ones: %v", err)
	}
}

func TestBasicLoginLocksOutAnAddressTryingManyNames(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for i := range 20 {
		if _, err := w.basic("name"+string(rune('a'+i)), "falsch"); err == nil {
			t.Fatal("an unknown name passed")
		}
	}
	var wait *accounts.TooManyTries
	if _, err := w.basic("anna", strong); !errors.As(err, &wait) {
		t.Fatalf("the twenty first try from one address: %v", err)
	}
}

func TestAWebDAVClientWithAnOldPasswordDoesNotLockTheSignInPage(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for range 20 {
		_, _ = w.basic("anna", "falsch")
	}
	w.must(http.StatusOK, w.signIn("anna", strong, false))
}
