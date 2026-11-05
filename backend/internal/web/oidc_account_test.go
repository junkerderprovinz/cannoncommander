package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const testIssuer = "https://id.example.org/realms/home"

// oidcSetup readies a server that signs in through OIDC, with an admin group
// and the groups that may sign in.
func oidcSetup(t *testing.T, adminGroup string, userGroups ...string) {
	t.Helper()
	setupTestEnv(t)
	accounts.UseLogin(sessionLogin{})
	cfg := &settings.Config.Auth.Methods.OidcAuth
	saved := *cfg
	t.Cleanup(func() { *cfg = saved })
	cfg.IssuerUrl, cfg.GroupsClaim, cfg.AdminGroup, cfg.UserGroups = testIssuer, "groups", adminGroup, userGroups
}

// makeOidcAccount creates an account that signs in through OIDC and has not
// signed in since.
func makeOidcAccount(t *testing.T, name string, admin bool) users.User {
	t.Helper()
	u := users.User{FrontendUser: users.FrontendUser{Username: name, LoginMethod: users.LoginMethodOidc}}
	if err := state.CreateUser(&u, ""); err != nil {
		t.Fatal(err)
	}
	u.Permissions.Admin = admin
	if err := state.UpdateUser(&u, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	return u
}

func oidcSignIn(t *testing.T, subject, name string, groups []string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	status, err := loginWithOidcUser(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil), oidcIdentity{issuer: testIssuer, subject: subject, name: name, groups: groups}, "/")
	if err != nil {
		t.Logf("sign in: %v", err)
	}
	if status == 0 {
		status = rec.Code
	}
	return status
}

func user(t *testing.T, name string) users.User {
	t.Helper()
	u, err := state.GetUserByUsername(name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAnOidcLoginDoesNotChangeTheAdminRightOfAPasswordAccount(t *testing.T) {
	oidcSetup(t, "admins")
	for _, admin := range []bool{true, false} {
		name := "lena"
		if admin {
			name = "chef"
		}
		u := users.User{FrontendUser: users.FrontendUser{Username: name, LoginMethod: users.LoginMethodPassword}}
		if err := state.CreateUser(&u, "korrekt-pferd-batterie"); err != nil {
			t.Fatal(err)
		}
		u.Permissions.Admin = admin
		if err := state.UpdateUser(&u, "", "permissions"); err != nil {
			t.Fatal(err)
		}
		groups := []string{"family"}
		if !admin {
			groups = []string{"admins"}
		}
		if status := oidcSignIn(t, "sub-"+name, name, groups); status != http.StatusUnauthorized {
			t.Fatalf("%s: the sign in went through on a password account: %d", name, status)
		}
		if got := user(t, name).Permissions.Admin; got != admin {
			t.Errorf("%s: a refused sign in changed the admin right to %v", name, got)
		}
	}
}

func TestAnAdminMadeByHandStaysAdminWithoutAnAdminGroup(t *testing.T) {
	oidcSetup(t, "")
	makeOidcAccount(t, "mara", true)

	if status := oidcSignIn(t, "sub-mara", "mara", []string{"family"}); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if !user(t, "mara").Permissions.Admin {
		t.Fatal("a sign in without an admin group took the admin right away")
	}
}

func TestAMissingGroupsClaimLeavesAdminAndGroupsAlone(t *testing.T) {
	oidcSetup(t, "admins")
	makeOidcAccount(t, "mara", true)
	if err := state.AddUserToGroup("family", "mara"); err != nil {
		t.Fatal(err)
	}

	if status := oidcSignIn(t, "sub-mara", "mara", nil); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if !user(t, "mara").Permissions.Admin {
		t.Fatal("a sign in without groups took the admin right away")
	}
	if got := state.GetUserGroups("mara"); len(got) != 1 || got[0] != "family" {
		t.Fatalf("groups = %v", got)
	}

	if status := oidcSignIn(t, "sub-mara", "mara", []string{"family"}); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if user(t, "mara").Permissions.Admin {
		t.Fatal("the admin group is set and mara is not in it, so she is no admin any more")
	}
}

func TestAnOidcAccountFollowsItsSubjectAndNotItsName(t *testing.T) {
	oidcSetup(t, "")
	if status := oidcSignIn(t, "sub-mara", "mara", nil); status != http.StatusFound {
		t.Fatalf("first sign in: %d", status)
	}
	mara := user(t, "mara")
	if status := oidcSignIn(t, "sub-mara", "mara.k", nil); status != http.StatusFound {
		t.Fatalf("sign in under the new name: %d", status)
	}
	if _, err := state.GetUserByUsername("mara.k"); err == nil {
		t.Fatal("a new name at the provider made a second account")
	}
	if user(t, "mara").ID != mara.ID {
		t.Fatal("the account changed")
	}
}

func TestAnotherSubjectCannotTakeOverAnAccountByItsName(t *testing.T) {
	oidcSetup(t, "")
	if status := oidcSignIn(t, "sub-mara", "mara", nil); status != http.StatusFound {
		t.Fatalf("first sign in: %d", status)
	}
	if status := oidcSignIn(t, "sub-someone-else", "mara", nil); status != http.StatusConflict {
		t.Fatalf("a second subject with the same name: %d", status)
	}
}

func TestAnAccountFromBeforeTheBindingIsBoundAtItsNextSignIn(t *testing.T) {
	oidcSetup(t, "")
	old := makeOidcAccount(t, "mara", false)
	if status := oidcSignIn(t, "sub-mara", "mara", nil); status != http.StatusFound {
		t.Fatalf("sign in of the old account: %d", status)
	}
	if user(t, "mara").ID != old.ID {
		t.Fatal("the old account was replaced")
	}
	if status := oidcSignIn(t, "sub-someone-else", "mara", nil); status != http.StatusConflict {
		t.Fatalf("a second subject took the bound account: %d", status)
	}
}

func TestLeavingTheAllowedGroupsEndsTokensAndSessions(t *testing.T) {
	oidcSetup(t, "", "family")
	if status := oidcSignIn(t, "sub-mara", "mara", []string{"family"}); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	mara := user(t, "mara")
	session, err := mintSessionToken(&mara, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	raw, tok, err := auth.MakeSignedTokenAPI(&mara, "skript", time.Hour, mara.Permissions, true)
	if err != nil {
		t.Fatal(err)
	}
	tok.Name, tok.Token = "skript", raw
	if err := state.AddUserToken("mara", tok); err != nil {
		t.Fatal(err)
	}
	if err := state.AddApiToken(raw, mara.ID); err != nil {
		t.Fatal(err)
	}

	if status := oidcSignIn(t, "sub-mara", "mara", []string{"friends"}); status != http.StatusForbidden {
		t.Fatalf("sign in outside the allowed groups: %d", status)
	}
	if !state.IsTokenRevoked(raw) {
		t.Error("the API token still works")
	}
	if state.SessionAlive(utils.HashSHA256(session)) {
		t.Error("the session still works")
	}
}
