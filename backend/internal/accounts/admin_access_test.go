package accounts_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

func (w *world) newToken(user, name string) accounts.NewToken {
	w.t.Helper()
	return decode[accounts.NewToken](w.t, w.must(http.StatusCreated, w.as(user, "POST", "/me/tokens", map[string]any{"name": name, "rights": "rw", "password": strong})))
}

func (w *world) givePasskey(name string) {
	w.t.Helper()
	u, err := state.GetUserByUsername(name)
	if err != nil {
		w.t.Fatal(err)
	}
	u.PasskeyCredentials = []users.WebAuthnCredential{{ID: "a2V5", Name: "Handy", Origin: "cannon.lan:8443", CreatedAt: time.Now().Unix()}}
	if err := state.UpdateUser(&u, "", "PasskeyCredentials"); err != nil {
		w.t.Fatal(err)
	}
}

func TestAdminsListAndRevokeTheTokensOfAUser(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	tok := w.newToken("anna", "skript")

	w.must(http.StatusForbidden, w.as("anna", "GET", "/admin/users/anna/tokens", nil))
	list := decode[struct{ Items []accounts.Token }](t, w.must(http.StatusOK, w.as("jdp", "GET", "/admin/users/anna/tokens", nil))).Items
	if len(list) != 1 || list[0].ID != tok.ID || list[0].Name != "skript" {
		t.Fatalf("list = %+v", list)
	}
	w.must(http.StatusNotFound, w.as("jdp", "DELETE", "/admin/users/anna/tokens/gibtsnicht", nil))
	w.must(http.StatusNoContent, w.as("jdp", "DELETE", "/admin/users/anna/tokens/"+tok.ID, nil))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, "X-Api", tok.Secret))
	if !state.IsTokenRevoked(tok.Secret) {
		t.Fatal("the token is not revoked")
	}
}

func TestAdminsListAndRemoveThePasskeysOfAUser(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.givePasskey("anna")

	list := decode[struct{ Items []accounts.Passkey }](t, w.must(http.StatusOK, w.as("jdp", "GET", "/admin/users/anna/passkeys", nil))).Items
	if len(list) != 1 || list[0].Name != "Handy" {
		t.Fatalf("list = %+v", list)
	}
	w.must(http.StatusNotFound, w.as("jdp", "DELETE", "/admin/users/anna/passkeys/gibtsnicht", nil))
	w.must(http.StatusNoContent, w.as("jdp", "DELETE", "/admin/users/anna/passkeys/a2V5", nil))
	if u, _ := state.GetUserByUsername("anna"); len(u.PasskeyCredentials) != 0 {
		t.Fatal("the passkey is still there")
	}
}

func TestAdminsListAndEndTheSessionsOfAUser(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as("anna", "GET", "/me", nil))
	w.must(http.StatusOK, w.as("jdp", "GET", "/me", nil))

	list := decode[struct{ Items []accounts.Session }](t, w.must(http.StatusOK, w.as("jdp", "GET", "/admin/users/anna/sessions", nil))).Items
	if len(list) != 1 || list[0].Current {
		t.Fatalf("list = %+v", list)
	}
	own := decode[struct{ Items []accounts.Session }](t, w.must(http.StatusOK, w.as("jdp", "GET", "/me/sessions", nil))).Items
	w.must(http.StatusNotFound, w.as("jdp", "DELETE", "/admin/users/anna/sessions/"+own[0].ID, nil))
	w.must(http.StatusNoContent, w.as("jdp", "DELETE", "/admin/users/anna/sessions/"+list[0].ID, nil))
	if !slices.Contains(w.login.ended, w.session("anna")) {
		t.Fatalf("ended = %v", w.login.ended)
	}
}

func TestAnAdminRevokesOnlyWithASession(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	anna := w.newToken("anna", "skript")
	admin := w.newToken("jdp", "verwaltung")
	for _, path := range []string{"/admin/users/anna/tokens/" + anna.ID, "/admin/users/anna/passkeys/a2V5", "/admin/users/anna/sessions/x"} {
		w.must(http.StatusForbidden, w.req("DELETE", path, nil, "X-Api", admin.Secret))
	}
	w.must(http.StatusForbidden, w.req("PATCH", "/admin/users/anna", map[string]any{"revoke": true}, "X-Api", admin.Secret))
}

func TestANewPasswordFromAnAdminCanRevokeTokensAndPasskeys(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	kept := w.newToken("anna", "bleibt")
	w.givePasskey("anna")

	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"password": "ein-ganz-neues-passwort"}))
	w.must(http.StatusOK, w.req("GET", "/me", nil, "X-Api", kept.Secret))

	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"password": "noch-ein-neues-passwort", "revoke": true}))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, "X-Api", kept.Secret))
	u, _ := state.GetUserByUsername("anna")
	if len(u.PasskeyCredentials) != 0 {
		t.Fatal("the passkey survived")
	}
	if last := w.login.endAll[len(w.login.endAll)-1]; last.user != u.ID || last.keep != "" {
		t.Fatalf("every session must end: %+v", last)
	}

	again := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "neu", "rights": "r", "password": "noch-ein-neues-passwort"})))
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"revoke": true}))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, "X-Api", again.Secret))
	w.must(http.StatusBadRequest, w.as("jdp", "PATCH", "/admin/users/jdp", map[string]any{"revoke": true}))
}
