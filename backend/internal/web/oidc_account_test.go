package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestAnOidcLoginDoesNotChangeTheAdminRightOfAPasswordAccount(t *testing.T) {
	setupTestEnv(t)
	admin := &users.User{FrontendUser: users.FrontendUser{Username: "chef", LoginMethod: users.LoginMethodPassword}}
	admin.Permissions.Admin = true
	if err := state.CreateUser(admin, "korrekt-pferd-batterie"); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(admin, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	if _, err := getOrCreateAuthenticatedUser("chef", users.LoginMethodOidc, new(false), nil); err == nil {
		t.Fatal("the login went through on an account with another login method")
	}
	got, err := state.GetUserByUsername("chef")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Permissions.Admin {
		t.Error("a login that was refused took the admin right away")
	}

	plain := &users.User{FrontendUser: users.FrontendUser{Username: "lena", LoginMethod: users.LoginMethodPassword}}
	if err := state.CreateUser(plain, "korrekt-pferd-batterie"); err != nil {
		t.Fatal(err)
	}
	if _, err := getOrCreateAuthenticatedUser("lena", users.LoginMethodOidc, new(true), []string{"admins"}); err == nil {
		t.Fatal("the login went through on an account with another login method")
	}
	got, err = state.GetUserByUsername("lena")
	if err != nil {
		t.Fatal(err)
	}
	if got.Permissions.Admin {
		t.Error("a login that was refused made a password account admin")
	}
}

// oidcAccount creates an account that signs in through OIDC.
func oidcAccount(t *testing.T, name string, admin bool) users.User {
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

func oidcSignIn(t *testing.T, name string, groups []string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	status, err := loginWithOidcUser(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil), name, groups, "/")
	if err != nil {
		t.Logf("sign in: %v", err)
	}
	if status == 0 {
		status = rec.Code
	}
	return status
}

func isAdmin(t *testing.T, name string) bool {
	t.Helper()
	u, err := state.GetUserByUsername(name)
	if err != nil {
		t.Fatal(err)
	}
	return u.Permissions.Admin
}

func TestAnAdminMadeByHandStaysAdminWithoutAnAdminGroup(t *testing.T) {
	setupTestEnv(t)
	cfg := &settings.Config.Auth.Methods.OidcAuth
	saved := *cfg
	t.Cleanup(func() { *cfg = saved })
	cfg.AdminGroup, cfg.GroupsClaim = "", "groups"
	oidcAccount(t, "mara", true)

	if status := oidcSignIn(t, "mara", []string{"family"}); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if !isAdmin(t, "mara") {
		t.Fatal("a sign in without an admin group took the admin right away")
	}
}

func TestAMissingGroupsClaimLeavesAdminAndGroupsAlone(t *testing.T) {
	setupTestEnv(t)
	cfg := &settings.Config.Auth.Methods.OidcAuth
	saved := *cfg
	t.Cleanup(func() { *cfg = saved })
	cfg.AdminGroup, cfg.GroupsClaim = "admins", "groups"
	oidcAccount(t, "mara", true)
	if err := state.AddUserToGroup("family", "mara"); err != nil {
		t.Fatal(err)
	}

	if status := oidcSignIn(t, "mara", nil); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if !isAdmin(t, "mara") {
		t.Fatal("a sign in without groups took the admin right away")
	}
	if got := state.GetUserGroups("mara"); len(got) != 1 || got[0] != "family" {
		t.Fatalf("groups = %v", got)
	}

	if status := oidcSignIn(t, "mara", []string{"family"}); status != http.StatusFound {
		t.Fatalf("sign in: %d", status)
	}
	if isAdmin(t, "mara") {
		t.Fatal("the admin group is set and mara is not in it, so she is no admin any more")
	}
}
