package accounts_test

import (
	"net/http"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

func TestANewTokenNeedsTheCurrentPassword(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw"}))
	w.must(http.StatusForbidden, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw", "password": "geraten-geraten"}))
	w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw", "password": strong}))
}

func TestANewPasskeyNeedsTheCurrentPassword(t *testing.T) {
	w := newWorld(t)
	passkeysAtCannon(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop"}))
	w.must(http.StatusForbidden, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop", "password": "geraten-geraten"}))
	w.must(http.StatusOK, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop", "password": strong}))
}

func TestWrongPasswordsBlockNewCredentials(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for range 5 {
		w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw", "password": "geraten-geraten"})
	}
	w.must(http.StatusTooManyRequests, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw", "password": strong}))
	w.must(http.StatusTooManyRequests, w.as("anna", "PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))
}

func TestAnAccountWithoutPasswordMakesTokensRightAfterSigningIn(t *testing.T) {
	w := newWorld(t)
	mara := users.User{FrontendUser: users.FrontendUser{Username: "mara", LoginMethod: users.LoginMethodOidc}}
	if err := state.CreateUser(&mara, ""); err != nil {
		t.Fatal(err)
	}
	w.login.signedIn = time.Now().Add(-10 * time.Minute)
	rec := w.must(http.StatusForbidden, w.as("mara", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "r"}))
	if p := decode[v1.Problem](t, rec); p.Type != "fresh-sign-in" {
		t.Fatalf("problem = %+v", p)
	}
	w.login.signedIn = time.Now().Add(-time.Minute)
	w.must(http.StatusCreated, w.as("mara", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "r"}))
}
