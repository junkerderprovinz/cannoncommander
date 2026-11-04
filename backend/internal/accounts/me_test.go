package accounts_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

func TestMeShowsTheOwnAccount(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user/fotos")
	me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil)))
	if me.Name != "anna" || me.Full != "Anna" || me.Role != "user" || me.Scope != "/mnt/user/fotos" || me.Twofa || me.Recovery != 0 || me.Token != nil {
		t.Fatalf("me = %+v", me)
	}
	// The first request of a session counts as the last login.
	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil))); me.Last == nil {
		t.Fatal("a request of a session must set the last login")
	}
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil))
}

func TestChangingThePasswordEndsTheOtherSessions(t *testing.T) {
	w := newWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	put := func(current, next string) int {
		return w.as("anna", "PUT", "/me/password", map[string]string{"current": current, "next": next}).Code
	}
	if code := put("falsch-falsch-falsch", "ein-ganz-neues-passwort"); code != http.StatusForbidden {
		t.Fatalf("wrong current password: %d", code)
	}
	if code := put(strong, "kurz"); code != http.StatusBadRequest {
		t.Fatalf("short password: %d", code)
	}
	if code := put(strong, strong); code != http.StatusBadRequest {
		t.Fatalf("the same password: %d", code)
	}
	got := decode[struct{ Ended int }](t, w.must(http.StatusOK, w.as("anna", "PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"})))
	if got.Ended != 2 {
		t.Fatalf("ended = %d", got.Ended)
	}
	if last := w.login.endAll[len(w.login.endAll)-1]; last.user != anna.ID || last.keep != w.session("anna") {
		t.Fatalf("this session must stay: %+v", last)
	}
	fresh, _ := state.GetUserByID(anna.ID)
	if utils.CheckPwd("ein-ganz-neues-passwort", fresh.Password) != nil {
		t.Fatal("the new password did not take")
	}
	if err := w.passwordLogin("anna", "ein-ganz-neues-passwort", ""); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
}

func TestWrongPasswordsBlockThePasswordChange(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for range 5 {
		w.as("anna", "PUT", "/me/password", map[string]string{"current": "geraten-geraten", "next": "ein-ganz-neues-passwort"})
	}
	w.must(http.StatusTooManyRequests, w.as("anna", "PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))
}

func TestTokensListWithoutTheirSecret(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	made := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "backup-skript", "rights": "r", "days": 30})))
	if made.Secret == "" || made.Expires == nil || made.Rights != "r" || made.ID == "" {
		t.Fatalf("made = %+v", made)
	}
	forever := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "nas", "rights": "rw", "days": nil})))
	if forever.Expires != nil {
		t.Fatalf("a token for always shows an expiry: %+v", forever)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "nas", "rights": "rw"}))
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "x", "rights": "admin"}))
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "", "rights": "r"}))
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "WEB_TOKEN_x", "rights": "r"}))

	w.must(http.StatusOK, w.req("GET", "/me", nil, "X-Api", made.Secret))
	rec := w.must(http.StatusOK, w.as("anna", "GET", "/me/tokens", nil))
	if strings.Contains(rec.Body.String(), made.Secret) {
		t.Fatal("the list gives the secret away")
	}
	list := decode[struct{ Items []accounts.Token }](t, rec).Items
	if len(list) != 2 || list[0].Name != "backup-skript" || list[0].Last == nil || list[1].Last != nil {
		t.Fatalf("list = %+v", list)
	}
}

func TestAReadTokenOnlyReads(t *testing.T) {
	w := newWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	read := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "lesen", "rights": "r"}))).Secret
	write := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "schreiben", "rights": "rw"}))).Secret

	me := decode[accounts.Me](t, w.must(http.StatusOK, w.req("GET", "/me", nil, "X-Api", read)))
	if me.Token == nil || me.Token.Name != "lesen" || me.Token.Rights != "r" {
		t.Fatalf("me with a token = %+v", me)
	}
	rec := w.req("DELETE", "/me/sessions/x", nil, "X-Api", read)
	if rec.Code != http.StatusForbidden || !strings.Contains(detail(t, rec), "may only read") {
		t.Fatalf("a read token wrote: %d %s", rec.Code, rec.Body)
	}
	if rec := w.req("DELETE", "/me/sessions/x", nil, "X-Api", write); rec.Code != http.StatusNotFound {
		t.Fatalf("a write token may write: %d %s", rec.Code, rec.Body)
	}

	fresh, _ := state.GetUserByID(anna.ID)
	name, _ := state.TokenNameForRawToken(&fresh, read)
	if !fresh.Tokens[name].ReadOnly {
		t.Fatal("the stored token lost its rights")
	}
	fresh.RestrictToReading()
	if _, err := w.fs.Check(&fresh, "/mnt/user/fotos", ccfs.Modify); err == nil {
		t.Fatal("the old API would let a read token write")
	}
	if _, err := w.fs.Check(&fresh, "/mnt/user/fotos", ccfs.View|ccfs.Download); err != nil {
		t.Fatalf("a read token reads: %v", err)
	}
}

func TestTokensCannotChangeLogins(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "voll", "rights": "rw"})))
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/me/tokens", map[string]any{"name": "mehr", "rights": "rw"}},
		{"DELETE", "/me/tokens/" + tok.ID, nil},
		{"PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}},
		{"POST", "/me/totp", map[string]any{}},
		{"DELETE", "/me/totp", map[string]any{"code": "123456"}},
		{"POST", "/me/passkeys", map[string]any{"name": "laptop"}},
		{"DELETE", "/me/passkeys/x", nil},
	} {
		if rec := w.req(c.method, c.path, c.body, "X-Api", tok.Secret); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with a token: %d %s", c.method, c.path, rec.Code, rec.Body)
		}
	}
}

func TestARevokedTokenStopsAtOnce(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.addUser("bert", "user", "/mnt/user")
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw"})))
	w.must(http.StatusOK, w.req("GET", "/me", nil, "X-Api", tok.Secret))
	w.must(http.StatusNotFound, w.as("bert", "DELETE", "/me/tokens/"+tok.ID, nil))
	w.must(http.StatusNoContent, w.as("anna", "DELETE", "/me/tokens/"+tok.ID, nil))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, "X-Api", tok.Secret))
	if !state.IsTokenRevoked(tok.Secret) {
		t.Fatal("the token is not revoked")
	}
	if list := decode[struct{ Items []accounts.Token }](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/tokens", nil))).Items; len(list) != 0 {
		t.Fatalf("list = %+v", list)
	}
}
