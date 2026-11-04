package state

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqldb"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// newAuthSigningKeyTestStore opens an isolated SQLite store and points the
// package-level sqlDb at it.
func newAuthSigningKeyTestStore(t *testing.T) *sqldb.SQLStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	store, _, err := sqldb.NewSQLStoreWithOptions(dbPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDb = store
	t.Cleanup(func() {
		_ = store.Close()
		sqlDb = nil
	})
	return store
}

// preserveAuthKey restores settings.Config.Auth.Key after the test so ordering
// between tests cannot leak a generated key.
func preserveAuthKey(t *testing.T) {
	t.Helper()
	original := settings.Config.Auth.Key
	t.Cleanup(func() { settings.Config.Auth.Key = original })
}

func TestInitAuthSigningKeyPersistsAndReloads(t *testing.T) {
	newAuthSigningKeyTestStore(t)
	preserveAuthKey(t)

	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	first := settings.Config.Auth.Key
	if first == "" {
		t.Fatal("expected generated key")
	}

	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	if settings.Config.Auth.Key != first {
		t.Fatalf("expected same key after reload, got different values")
	}
}

func TestInitAuthSigningKeyRejectsMismatchedConfiguredKey(t *testing.T) {
	newAuthSigningKeyTestStore(t)
	preserveAuthKey(t)

	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	stored := settings.Config.Auth.Key
	if stored == "" {
		t.Fatal("expected generated key")
	}

	// A config/env key that disagrees with the persisted key must abort startup
	// rather than silently minting tokens other nodes cannot verify.
	settings.Config.Auth.Key = "different-config-key"
	if err := InitAuthSigningKey(); err == nil {
		t.Fatal("expected error for config/env key mismatch with persisted key")
	}
	if settings.Config.Auth.Key != "different-config-key" {
		t.Fatalf("config key was mutated on failure: %q", settings.Config.Auth.Key)
	}

	// The persisted key remains authoritative: with no explicit key it loads.
	settings.Config.Auth.Key = ""
	if err := InitAuthSigningKey(); err != nil {
		t.Fatal(err)
	}
	if settings.Config.Auth.Key != stored {
		t.Fatalf("expected persisted key %q after reload, got %q", stored, settings.Config.Auth.Key)
	}
}

func TestInitAuthSigningKeyRejectsMalformedPersistedRow(t *testing.T) {
	store := newAuthSigningKeyTestStore(t)
	preserveAuthKey(t)

	// A corrupt row must abort startup instead of being overwritten with a new key.
	if _, err := store.DB().Exec(
		`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`,
		authSigningKeySetting, []byte("{not-json"),
	); err != nil {
		t.Fatal(err)
	}

	settings.Config.Auth.Key = "explicit-config-key"
	err := InitAuthSigningKey()
	if err == nil {
		t.Fatal("expected error for malformed persisted signing key")
	}
	if settings.Config.Auth.Key != "explicit-config-key" {
		t.Fatalf("config key was mutated on failure: %q", settings.Config.Auth.Key)
	}

	raw, getErr := store.GetSetting(authSigningKeySetting)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if string(raw) != "{not-json" {
		t.Fatalf("malformed row was overwritten: %q", string(raw))
	}
}

