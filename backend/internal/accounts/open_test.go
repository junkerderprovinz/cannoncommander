package accounts_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// openWorld is a world whose only account has no password yet.
func openWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	if err := state.RemoveSignIn(w.admin.ID); err != nil {
		t.Fatal(err)
	}
	if !state.OpenAccess() {
		t.Fatal("without a password the instance has to be open")
	}
	return w
}

func TestTheFirstPasswordNeedsNoOldOne(t *testing.T) {
	w := openWorld(t)

	w.must(http.StatusOK, w.req("PUT", "/me/password", map[string]string{"next": strong}, "X-User", w.admin.Username))
	if state.OpenAccess() {
		t.Fatal("a password closes the instance")
	}
	if err := w.passwordLogin(w.admin.Username, strong, ""); err != nil {
		t.Fatalf("sign in with the first password: %v", err)
	}
}

func TestTheFirstPasswordSignsTheBrowserIn(t *testing.T) {
	w := openWorld(t)

	rec := w.must(http.StatusOK, w.req("PUT", "/me/password", map[string]string{"next": strong}, "X-User", w.admin.Username))
	if rec.Header().Get("Set-Cookie") == "" {
		t.Fatal("the browser that set the password has to stay signed in")
	}
}

func TestRemovingThePasswordOpensTheInstance(t *testing.T) {
	w := newWorld(t)

	w.must(http.StatusOK, w.as(w.admin.Username, "DELETE", "/me/password", map[string]string{"current": strong}))
	if !state.OpenAccess() {
		t.Fatal("without the password the instance has to open")
	}
	u, err := state.GetUserByID(w.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Password != "" || u.OtpEnabled || len(u.PasskeyCredentials) > 0 {
		t.Fatalf("credentials left behind: password %v, otp %v, passkeys %d", u.Password != "", u.OtpEnabled, len(u.PasskeyCredentials))
	}
}

func TestRemovingThePasswordAsksForIt(t *testing.T) {
	w := newWorld(t)

	w.must(http.StatusForbidden, w.as(w.admin.Username, "DELETE", "/me/password", map[string]string{"current": "geraten-geraten"}))
	if state.OpenAccess() {
		t.Fatal("a wrong password must not open the instance")
	}
}

func TestThePasswordStaysWhileOthersSignIn(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")

	rec := w.must(http.StatusConflict, w.as(w.admin.Username, "DELETE", "/me/password", map[string]string{"current": strong}))
	if got := decode[struct{ Code string }](t, rec).Code; got != "othersSignIn" {
		t.Fatalf("code = %q, want othersSignIn", got)
	}
}

func TestWhileOpenNoOtherAccountGetsAPassword(t *testing.T) {
	w := openWorld(t)

	rec := w.must(http.StatusConflict, w.req("POST", "/admin/users", map[string]any{"name": "anna", "role": "user", "scope": "/mnt/user", "password": strong}, "X-User", w.admin.Username))
	if got := decode[struct{ Code string }](t, rec).Code; got != "ownPasswordFirst" {
		t.Fatalf("code = %q, want ownPasswordFirst", got)
	}
	if !state.OpenAccess() {
		t.Fatal("the instance has to stay open")
	}
}

func TestWhileOpenTokensWaitForAPassword(t *testing.T) {
	w := openWorld(t)

	rec := w.must(http.StatusConflict, w.req("POST", "/me/tokens", map[string]any{"name": "skript", "rights": "r"}, "X-User", w.admin.Username))
	if got := decode[struct{ Code string }](t, rec).Code; got != "ownPasswordFirst" {
		t.Fatalf("code = %q, want ownPasswordFirst", got)
	}
}

func TestMeTellsWhetherSignInIsOn(t *testing.T) {
	w := openWorld(t)

	open := decode[struct{ SignIn, HasPassword bool }](t, w.must(http.StatusOK, w.req("GET", "/me", nil, "X-User", w.admin.Username)))
	if open.SignIn || open.HasPassword {
		t.Fatalf("open instance reports %+v", open)
	}
	w.must(http.StatusOK, w.req("PUT", "/me/password", map[string]string{"next": strong}, "X-User", w.admin.Username))
	closed := decode[struct{ SignIn, HasPassword bool }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/me", nil)))
	if !closed.SignIn || !closed.HasPassword {
		t.Fatalf("closed instance reports %+v", closed)
	}
}
