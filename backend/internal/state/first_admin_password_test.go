package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func startFresh(t *testing.T, adminPassword string) string {
	t.Helper()
	t.Setenv("CC_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../testdata/noauth.yaml")
	settings.Config.Auth.Methods.NoAuth = false
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = adminPassword
	settings.Config.Auth.AdminPassword = ""
	dir := t.TempDir()
	if _, err := Initialize(filepath.Join(dir, "cannoncommander.sqlite")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	return filepath.Join(dir, "first-admin-password")
}

func TestAFreshInstallOpensWithoutAPassword(t *testing.T) {
	file := startFresh(t, "")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Password != "" || admin.RequirePasswordChange {
		t.Fatalf("the first admin got a password: %q, change due %v", admin.Password, admin.RequirePasswordChange)
	}
	if !OpenAccess() {
		t.Fatal("a fresh install has to open without a sign in")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want no password file, stat says %v", err)
	}
}

func TestTheDefaultAdminPasswordCountsAsNone(t *testing.T) {
	startFresh(t, "admin")

	if !OpenAccess() {
		t.Fatal("the placeholder password admin must not close a fresh install")
	}
}

func TestAConfiguredAdminPasswordStartsClosed(t *testing.T) {
	file := startFresh(t, "configured-secret-1")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := utils.CheckPwd("configured-secret-1", admin.Password); err != nil {
		t.Fatalf("the configured password does not sign the admin in: %v", err)
	}
	if admin.RequirePasswordChange {
		t.Fatal("a configured password is the admin's choice and needs no change")
	}
	if OpenAccess() {
		t.Fatal("an install with a configured admin password must ask for it")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want no password file, stat says %v", err)
	}
}

func TestSettingAPasswordClosesTheInstance(t *testing.T) {
	startFresh(t, "")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(&admin, "a-new-password-123", "password"); err != nil {
		t.Fatal(err)
	}
	if OpenAccess() {
		t.Fatal("an admin with a password has to sign in")
	}
}

func TestAPasskeyAloneClosesTheInstance(t *testing.T) {
	startFresh(t, "")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	admin.PasskeyCredentials = []users.WebAuthnCredential{{ID: "laptop", Name: "Laptop"}}
	if err := UpdateUser(&admin, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}
	if OpenAccess() {
		t.Fatal("an admin with a passkey has to sign in")
	}
	admin.PasskeyCredentials = nil
	if err := UpdateUser(&admin, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}
	if !OpenAccess() {
		t.Fatal("without the passkey nobody can sign in, so the instance opens")
	}
}

func TestDeletingTheLastAccountWithAPasswordOpensTheInstance(t *testing.T) {
	startFresh(t, "")

	alice := &users.User{FrontendUser: users.FrontendUser{Username: "alice", LoginMethod: users.LoginMethodPassword}}
	if err := CreateUser(alice, "first-password-123"); err != nil {
		t.Fatal(err)
	}
	if OpenAccess() {
		t.Fatal("an account with a password closes the instance")
	}
	if err := DeleteUser(alice.ID); err != nil {
		t.Fatal(err)
	}
	if !OpenAccess() {
		t.Fatal("with the last password gone the instance opens")
	}
}

func TestAPasswordOfASingleSignOnAccountDoesNotCount(t *testing.T) {
	startFresh(t, "")

	bob := &users.User{FrontendUser: users.FrontendUser{Username: "bob", LoginMethod: users.LoginMethodOidc}}
	if err := CreateUser(bob, "first-password-123"); err != nil {
		t.Fatal(err)
	}
	if !OpenAccess() {
		t.Fatal("an account that signs in elsewhere cannot sign in here")
	}
}

func TestSingleSignOnKeepsTheInstanceClosed(t *testing.T) {
	startFresh(t, "")
	settings.Config.Auth.Methods.OidcAuth.Enabled = true

	if OpenAccess() {
		t.Fatal("with single sign on people sign in there")
	}
}

func TestNoauthIsOpenWithPasswords(t *testing.T) {
	startFresh(t, "configured-secret-1")
	settings.Config.Auth.Methods.NoAuth = true

	if !OpenAccess() {
		t.Fatal("noauth lets everyone in")
	}
}

func TestAnOlderInstallKeepsItsPassword(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cannoncommander.sqlite")
	t.Setenv("CC_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../testdata/noauth.yaml")
	settings.Config.Auth.Methods.NoAuth = false
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	settings.Config.Auth.AdminPassword = ""
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	admin.RequirePasswordChange = true
	if err := UpdateUser(&admin, "speakable-word-ab3cd-ef", "password", "requirePasswordChange"); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	leftover := filepath.Join(dir, "first-admin-password")
	if err := os.WriteFile(leftover, []byte("speakable-word-ab3cd-ef\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	if OpenAccess() {
		t.Fatal("an install whose admin has a password stays closed")
	}
	admin, err = GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(&admin, "a-new-password-123", "password", "requirePasswordChange"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Fatalf("want the leftover password file gone, stat says %v", err)
	}
}
