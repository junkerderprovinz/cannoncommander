package sqldb

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func openAt(t *testing.T, path string) *SQLStore {
	t.Helper()
	store, _, err := NewSQLStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func armedStore(t *testing.T) (*SQLStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.db")
	store := openAt(t, path)
	u := &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "anna", LoginMethod: users.LoginMethodPassword}}
	u.Password = "$2a$10$old"
	if err := store.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := store.ArmLegacyFingerprint(); err != nil {
		t.Fatal(err)
	}
	return store, path
}

func lowerSchemaVersion(t *testing.T, store *SQLStore) {
	t.Helper()
	if _, err := store.DB().Exec(`UPDATE schema_version SET version = ?`, currentSchemaVersion-1); err != nil {
		t.Fatal(err)
	}
}

func TestAnOlderBuildOpeningTheFileTripsTheWire(t *testing.T) {
	store, path := armedStore(t)
	if store.OlderBuildOpened() {
		t.Fatal("tripped on the start that armed it")
	}
	lowerSchemaVersion(t, store)
	store.Close()

	again := openAt(t, path)
	if !again.OlderBuildOpened() {
		t.Fatal("a lowered schema version after an armed start went unnoticed")
	}
	again.Close()
	if openAt(t, path).OlderBuildOpened() {
		t.Fatal("the wire stays tripped once the version is back up")
	}
}

func TestTheFirstStartAfterTheOlderBuildTripsNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.db")
	store := openAt(t, path)
	lowerSchemaVersion(t, store)
	store.Close()
	if openAt(t, path).OlderBuildOpened() {
		t.Fatal("an upgrade that never ran the newer build before counts as a rollback")
	}
}

func TestOwnWritesKeepTheFingerprint(t *testing.T) {
	store, _ := armedStore(t)
	u, err := store.GetUserByID(42)
	if err != nil {
		t.Fatal(err)
	}
	u.Password = "$2a$10$new"
	u.Tokens = map[string]users.AuthToken{"backup": {Name: "backup", Token: "a.b.c"}}
	if err = store.UpdateUser(u); err != nil {
		t.Fatal(err)
	}
	if err = store.PersistImmediateTokenRevocation("deadbeef"); err != nil {
		t.Fatal(err)
	}
	other := &users.User{ID: 7, FrontendUser: users.FrontendUser{Username: "bert"}}
	if err = store.CreateUser(other); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteUserByID(7); err != nil {
		t.Fatal(err)
	}
	stored, current, err := store.LegacyFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if stored != current {
		t.Fatal("a write of this build left the fingerprint behind")
	}
}

func TestTheFingerprintSeesWhatTheOlderBuildChanges(t *testing.T) {
	changes := map[string]string{
		"password":   `UPDATE users SET user_data = json_set(user_data, '$.password', '$2a$10$leaked') WHERE user_id = '42'`,
		"role":       `UPDATE users SET perm_admin = 1 WHERE user_id = '42'`,
		"rename":     `UPDATE users SET username = 'anne' WHERE user_id = '42'`,
		"new user":   `INSERT INTO users (user_id, username, user_data) VALUES ('9', 'carl', '{"loginMethod":"password"}')`,
		"delete":     `DELETE FROM users WHERE user_id = '42'`,
		"two-factor": `UPDATE users SET user_data = json_set(user_data, '$.totpSecret', 'JBSWY3DPEHPK3PXP') WHERE user_id = '42'`,
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			store, _ := armedStore(t)
			if _, err := store.DB().Exec(change); err != nil {
				t.Fatal(err)
			}
			stored, current, err := store.LegacyFingerprint()
			if err != nil {
				t.Fatal(err)
			}
			if stored == current {
				t.Fatal("the fingerprint missed it")
			}
		})
	}
}

func TestATokenRevokedByTheOlderBuildChangesTheFingerprint(t *testing.T) {
	store, _ := armedStore(t)
	u, _ := store.GetUserByID(42)
	u.Tokens = map[string]users.AuthToken{"backup": {Name: "backup", Token: "a.b.c"}}
	if err := store.UpdateUser(u); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO revoked_tokens (token_hash, revoked_at) VALUES (?, 0)`, tokenDigest("a.b.c")); err != nil {
		t.Fatal(err)
	}
	stored, current, _ := store.LegacyFingerprint()
	if stored == current {
		t.Fatal("a revoked API token went unnoticed")
	}
}

func TestSessionsOfTheOlderBuildLeaveTheFingerprintAlone(t *testing.T) {
	store, _ := armedStore(t)
	if _, err := store.DB().Exec(`INSERT INTO hashed_tokens (token_hash, user_id, is_session) VALUES ('s1', '42', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO revoked_tokens (token_hash, revoked_at) VALUES ('s0', 0)`); err != nil {
		t.Fatal(err)
	}
	stored, current, _ := store.LegacyFingerprint()
	if stored != current {
		t.Fatal("signing in and out on the older build tripped the fingerprint")
	}
}
