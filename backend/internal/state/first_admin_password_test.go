package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func startFresh(t *testing.T, adminPassword string) string {
	t.Helper()
	t.Setenv("CC_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../testdata/noauth.yaml")
	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = adminPassword
	settings.Config.Auth.AdminPassword = ""
	dir := t.TempDir()
	if _, err := Initialize(filepath.Join(dir, "cannoncommander.sqlite")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	return filepath.Join(dir, "first-admin-password")
}

func TestFirstStartKeepsTheGeneratedAdminPasswordBesideTheDatabase(t *testing.T) {
	file := startFresh(t, "")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := utils.CheckPwd(strings.TrimSpace(string(data)), admin.Password); err != nil {
		t.Fatalf("the password in the file does not sign the admin in: %v", err)
	}
}

func TestConfiguredAdminPasswordStaysOffTheDisk(t *testing.T) {
	file := startFresh(t, "configured-secret-1")

	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want no password file, stat says %v", err)
	}
}

func TestChangingTheAdminPasswordRemovesTheFile(t *testing.T) {
	file := startFresh(t, "")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(&admin, "a-new-password-123", "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want the password file gone, stat says %v", err)
	}
}

func TestAnotherUsersPasswordChangeKeepsTheFile(t *testing.T) {
	file := startFresh(t, "")

	alice := &users.User{FrontendUser: users.FrontendUser{Username: "alice"}}
	if err := CreateUser(alice, "first-password-123"); err != nil {
		t.Fatal(err)
	}
	stored, err := GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateUser(&stored, "second-password-123", "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("want the password file kept: %v", err)
	}
}
