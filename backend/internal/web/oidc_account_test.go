package web

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
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
	if _, err := getOrCreateAuthenticatedUser("chef", users.LoginMethodOidc, false, nil, true); err == nil {
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
	if _, err := getOrCreateAuthenticatedUser("lena", users.LoginMethodOidc, true, []string{"admins"}, true); err == nil {
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
