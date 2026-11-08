package accounts_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// passkeysAtCannon points the passkeys at cannon.lan, where the fake login
// says every request comes in, with password sign in on as passkeys need it.
func passkeysAtCannon(t *testing.T) {
	t.Helper()
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	settings.Config.Auth.Methods.PasskeyAuth = settings.PasskeyAuthConfig{RPID: "cannon.lan", RPOrigins: []string{"https://cannon.lan:8443"}}
	if err := auth.InitWebAuthn(state.Default()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.PasskeyAuth = settings.PasskeyAuthConfig{}
		if err := auth.InitWebAuthn(state.Default()); err != nil {
			t.Error(err)
		}
	})
}

func blocked(t *testing.T, w *world, user string) string {
	t.Helper()
	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as(user, "GET", "/me/passkeys", nil)))
	if list.Blocked == nil {
		return ""
	}
	return list.Blocked.Code
}

func TestPasskeysNeedNoSwitch(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	if why := blocked(t, w, "anna"); why != "" {
		t.Fatalf("passkeys are blocked: %s", why)
	}
	w.must(http.StatusOK, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop"}))
	if !decode[accounts.SignInOptions](t, w.must(http.StatusOK, w.req("GET", "/auth/options", nil))).Passkey {
		t.Error("the sign in page offers no passkey")
	}
	w.must(http.StatusOK, w.req("POST", "/auth/passkey/options", nil))
}

func TestPasskeysNeedAHostName(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.login.onIP = true
	if why := blocked(t, w, "anna"); why != "passkeyNeedsHostName" {
		t.Errorf("on an IP address passkeys are blocked by %q", why)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop"}))
	if decode[accounts.SignInOptions](t, w.must(http.StatusOK, w.req("GET", "/auth/options", nil))).Passkey {
		t.Error("the sign in page offers a passkey on an IP address")
	}
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey/options", nil))
}

func TestAPasskeyComesWithAPassword(t *testing.T) {
	w := signInWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for name, method := range map[string]users.LoginMethod{"ohne": users.LoginMethodPassword, "mara": users.LoginMethodOidc} {
		u := users.User{FrontendUser: users.FrontendUser{Username: name, LoginMethod: method}}
		if err := state.CreateUser(&u, ""); err != nil {
			t.Fatal(err)
		}
	}
	if why := blocked(t, w, "ohne"); why != "passwordFirst" {
		t.Errorf("an account without a password is blocked by %q", why)
	}
	w.must(http.StatusConflict, w.as("ohne", "POST", "/me/passkeys", map[string]any{"name": "Laptop"}))
	if why := blocked(t, w, "mara"); why != "passkeysElsewhere" {
		t.Errorf("an account of the single sign on is blocked by %q", why)
	}

	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	if why := blocked(t, w, "anna"); why != "noPasswords" {
		t.Errorf("on a server without passwords passkeys are blocked by %q", why)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop"}))
	if decode[accounts.SignInOptions](t, w.must(http.StatusOK, w.req("GET", "/auth/options", nil))).Passkey {
		t.Error("the sign in page offers a passkey on a server without passwords")
	}
	w.must(http.StatusConflict, w.req("POST", "/auth/passkey/options", nil))
}

func TestRegisteringAPasskeyStartsWithOptions(t *testing.T) {
	w := newWorld(t)
	passkeysAtCannon(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	anna.PasskeyCredentials = []users.WebAuthnCredential{{ID: "a2V5", Name: "Handy", Origin: "cannon.lan:8443", CreatedAt: time.Now().Unix()}}
	if err := state.UpdateUser(&anna, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}

	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": " "}))
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Handy"}))
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop", "credential": map[string]any{"id": "x"}}))

	step := decode[struct {
		Options struct {
			PublicKey struct {
				Challenge string
				RP        struct{ ID string }
			}
		}
	}](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop"})))
	if step.Options.PublicKey.Challenge == "" || step.Options.PublicKey.RP.ID != "cannon.lan" {
		t.Fatalf("options = %+v", step)
	}
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop", "credential": map[string]any{"id": "x", "type": "public-key"}}))
	// A failed second step uses the registration up.
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"password": strong, "name": "Laptop", "credential": map[string]any{"id": "x"}}))

	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/passkeys", nil)))
	if list.Blocked != nil || len(list.Items) != 1 || list.Items[0].Origin != "cannon.lan:8443" || list.Items[0].Last != nil {
		t.Fatalf("list = %+v", list)
	}
	w.must(http.StatusNotFound, w.as("anna", "DELETE", "/me/passkeys/gibtsnicht", nil))
	w.must(http.StatusNoContent, w.as("anna", "DELETE", "/me/passkeys/a2V5", nil))
	if fresh, _ := state.GetUserByUsername("anna"); len(fresh.PasskeyCredentials) != 0 {
		t.Fatal("the passkey is still there")
	}
}

func TestTheActivityLogShowsWhoDidWhat(t *testing.T) {
	w := newWorld(t)
	state.InitActivityRecorder(settings.Database{})
	w.addUser("anna", "user", "/mnt/user")
	activity.Stop()

	page := decode[accounts.ActivityPage](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/activity?eventType=userCreate", nil)))
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	e := page.Items[0]
	if e.Who != "jdp" || e.What != "userCreate" || e.Obj != "anna" || time.Since(e.T) > time.Minute {
		t.Fatalf("entry = %+v", e)
	}
	raw, _ := json.Marshal(e)
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil || wire["t"] == nil || wire["who"] == nil {
		t.Fatalf("wire = %s", raw)
	}
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "GET", "/admin/activity?eventType=gibtsnicht", nil))
	w.must(http.StatusForbidden, w.as("anna", "GET", "/admin/activity", nil))
}
