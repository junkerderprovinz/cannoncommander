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

func turnOnPasskeys(t *testing.T) {
	t.Helper()
	settings.Config.Auth.Methods.PasskeyAuth = settings.PasskeyAuthConfig{Enabled: true, RPID: "cannon.lan", RPOrigins: []string{"https://cannon.lan:8443"}}
	if err := auth.InitWebAuthn(state.Default()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.PasskeyAuth.Enabled = false
		if err := auth.InitWebAuthn(state.Default()); err != nil {
			t.Error(err)
		}
	})
}

func TestPasskeysNeedTheServerToTakeThem(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/passkeys", nil)))
	if list.Enabled || len(list.Items) != 0 {
		t.Fatalf("list = %+v", list)
	}
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop"}))
}

func TestRegisteringAPasskeyStartsWithOptions(t *testing.T) {
	w := newWorld(t)
	turnOnPasskeys(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	anna.PasskeyCredentials = []users.WebAuthnCredential{{ID: "a2V5", Name: "Handy", Origin: "cannon.lan:8443", CreatedAt: time.Now().Unix()}}
	if err := state.UpdateUser(&anna, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}

	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": " "}))
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Handy"}))
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop", "credential": map[string]any{"id": "x"}}))

	step := decode[struct {
		Options struct {
			PublicKey struct {
				Challenge string
				RP        struct{ ID string }
			}
		}
	}](t, w.must(http.StatusOK, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop"})))
	if step.Options.PublicKey.Challenge == "" || step.Options.PublicKey.RP.ID != "cannon.lan" {
		t.Fatalf("options = %+v", step)
	}
	w.must(http.StatusBadRequest, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop", "credential": map[string]any{"id": "x", "type": "public-key"}}))
	// A failed second step uses the registration up.
	w.must(http.StatusConflict, w.as("anna", "POST", "/me/passkeys", map[string]any{"name": "Laptop", "credential": map[string]any{"id": "x"}}))

	list := decode[accounts.PasskeyList](t, w.must(http.StatusOK, w.as("anna", "GET", "/me/passkeys", nil)))
	if !list.Enabled || len(list.Items) != 1 || list.Items[0].Origin != "cannon.lan:8443" || list.Items[0].Last != nil {
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
