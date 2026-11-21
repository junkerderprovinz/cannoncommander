package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestAFreshInstallStartsTheAdminWithoutAPassword(t *testing.T) {
	file := startFresh(t, "")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Password != "" || admin.RequirePasswordChange {
		t.Fatalf("the first admin got a password: %q, change due %v", admin.Password, admin.RequirePasswordChange)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want no password file, stat says %v", err)
	}
}

func TestTheDefaultAdminPasswordCountsAsNone(t *testing.T) {
	startFresh(t, "admin")

	admin, err := GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Password != "" {
		t.Fatal("the placeholder password admin became a password")
	}
}

func TestAConfiguredAdminPasswordGoesToTheAdmin(t *testing.T) {
	startFresh(t, "configured-secret-1")

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
}

func TestTheServerStaysClosedUntilTheSignInAnswers(t *testing.T) {
	startFresh(t, "")
	if OpenAccess() {
		t.Fatal("open before the sign-in said who can sign in")
	}
	open := true
	UseSignIn(func() bool { return open })
	if !OpenAccess() {
		t.Fatal("the sign-in says nobody can sign in, yet the server asks")
	}
	open = false
	if OpenAccess() {
		t.Fatal("the sign-in has a password, yet the server is open")
	}
}

func TestNoauthIsOpenWhateverTheSignInSays(t *testing.T) {
	startFresh(t, "configured-secret-1")
	UseSignIn(func() bool { return false })
	settings.Config.Auth.Methods.NoAuth = true

	if !OpenAccess() {
		t.Fatal("noauth lets everyone in")
	}
}

func TestTheFirstPasswordFileGoesWhenTheAdminHasOne(t *testing.T) {
	file := startFresh(t, "")
	if err := os.WriteFile(file, []byte("speakable-word-ab3cd-ef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ForgetFirstAdminPassword()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("want the password file gone, stat says %v", err)
	}
}

func TestTheSealKeyIsMadeOnceAndKept(t *testing.T) {
	startFresh(t, "")
	first, err := SealKey()
	if err != nil {
		t.Fatal(err)
	}
	again, err := SealKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || string(first) != string(again) {
		t.Fatal("the sign-in key changed between two reads")
	}
}

// rolledBack starts this version once with an armed fingerprint, lets the
// older version change, and returns the database for the next start.
func rolledBack(t *testing.T, change string) string {
	t.Helper()
	t.Setenv("CC_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../testdata/noauth.yaml")
	settings.Config.Auth.Methods.NoAuth = false
	path := filepath.Join(t.TempDir(), "cannoncommander.sqlite")
	if _, err := Initialize(path); err != nil {
		t.Fatal(err)
	}
	if err := ArmLegacyFingerprint(); err != nil {
		t.Fatal(err)
	}
	db := MainDB()
	if _, err := db.Exec(`UPDATE schema_version SET version = version - 1`); err != nil {
		t.Fatal(err)
	}
	if change != "" {
		if _, err := db.Exec(change); err != nil {
			t.Fatal(err)
		}
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

const leakedPassword = `UPDATE users SET user_data = json_set(user_data, '$.password', '$2a$10$changedInTheOlderVersion')`

func TestARollbackThatChangedAccountsStopsTheStart(t *testing.T) {
	path := rolledBack(t, leakedPassword)
	_, err := Initialize(path)
	if err == nil {
		_ = Close()
		t.Fatal("started on accounts the older version changed")
	}
	if !strings.Contains(err.Error(), reimportEnv) {
		t.Fatalf("the refusal does not say how to go on: %v", err)
	}
}

func TestARollbackThatChangedNothingStarts(t *testing.T) {
	path := rolledBack(t, "")
	if _, err := Initialize(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	if Reimport() {
		t.Fatal("imports again with nothing changed")
	}
}

func TestTheReimportVariableActsOnce(t *testing.T) {
	path := rolledBack(t, leakedPassword)
	t.Setenv(reimportEnv, "1")
	if _, err := Initialize(path); err != nil {
		t.Fatal(err)
	}
	if !Reimport() {
		t.Fatal("the variable did not ask for the import")
	}
	// The import has not run, so the older version's change still differs.
	if _, err := MainDB().Exec(`UPDATE schema_version SET version = version - 1`); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(path); err == nil {
		_ = Close()
		t.Fatal("the variable left set imported a second time")
	}

	t.Setenv(reimportEnv, "")
	if _, err := Initialize(path); err == nil {
		_ = Close()
		t.Fatal("started without the variable on changed accounts")
	}
	t.Setenv(reimportEnv, "1")
	if _, err := Initialize(path); err != nil {
		t.Fatalf("the variable set again after a start without it: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	if !Reimport() {
		t.Fatal("set again, the variable did not ask for the import")
	}
}
