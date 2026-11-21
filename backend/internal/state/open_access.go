package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqldb"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// signInOpen is the sign-in's answer to whether nobody can sign in. The
// server counts as closed until it is connected.
var signInOpen atomic.Pointer[func() bool]

// UseSignIn connects the open state to the sign-in. open reports whether
// nobody can sign in.
func UseSignIn(open func() bool) { signInOpen.Store(&open) }

// OpenAccess reports whether every request acts as the admin without a sign
// in: under noauth, or while the sign-in has nobody who can sign in.
func OpenAccess() bool {
	if settings.Config.Auth.Methods.NoAuth {
		return true
	}
	open := signInOpen.Load()
	return open != nil && (*open)()
}

const (
	sealKeySetting  = "signin.sealKey"
	reimportSetting = "signin.reimported"
	reimportEnv     = "CC_SIGNIN_REIMPORT"
)

// reimport is whether this start imports the accounts again.
var reimport bool

// Reimport reports whether this start throws the sign-in's documents away
// and imports the accounts again, because CC_SIGNIN_REIMPORT asked for it
// after an older version changed them.
func Reimport() bool { return reimport }

// checkRollback runs before this version writes anything, so the tables the
// older version signs people in with are as that version left them.
func checkRollback() error {
	reimport = false
	asked := os.Getenv(reimportEnv) == "1"
	done, err := sqlDb.GetSetting(reimportSetting)
	if err != nil && !errors.Is(err, sqldb.ErrSettingNotFound) {
		return err
	}
	// The variable acts once. A start without it makes it count again.
	if !asked && done != nil {
		if err = sqlDb.DeleteSetting(reimportSetting); err != nil {
			return err
		}
	}
	if !sqlDb.OlderBuildOpened() {
		return nil
	}
	stored, current, err := sqlDb.LegacyFingerprint()
	if err != nil {
		return err
	}
	if stored == current {
		logger.Info("An older version of CannonCommander ran on this database and changed no accounts.")
		return nil
	}
	if asked && done == nil {
		if err := sqlDb.SaveSetting(reimportSetting, current); err != nil {
			return err
		}
		reimport = true
		logger.Warningf("%s=1: the accounts are imported again from what the older version left.", reimportEnv)
		return nil
	}
	if err := sqlDb.StayTripped(); err != nil {
		return err
	}
	if asked {
		return fmt.Errorf("an older version of CannonCommander changed accounts after this version ran, and %s=1 acted once already: start once without it, then set it again", reimportEnv)
	}
	return fmt.Errorf("an older version of CannonCommander changed accounts after this version ran. Start once with %s=1 to import them again, which ends every sign-in made since, or go back to the older version", reimportEnv)
}

// ArmLegacyFingerprint records how the tables the older version signs people
// in with look after the import, so a later start can tell whether that
// version changed them.
func ArmLegacyFingerprint() error { return sqlDb.ArmLegacyFingerprint() }

// InLegacyTx runs fn in a transaction on the main database that records the
// new state of the older version's sign-in tables with it.
func InLegacyTx(fn func(tx *sql.Tx) error) error { return sqlDb.InLegacyTx(fn) }

// SealKey returns the sign-in's own key, made once and kept in the
// database.
func SealKey() ([]byte, error) {
	raw, err := sqlDb.GetSetting(sealKeySetting)
	if errors.Is(err, sqldb.ErrSettingNotFound) {
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = sqlDb.SaveSetting(sealKeySetting, base64.StdEncoding.EncodeToString(key)); err != nil {
			return nil, err
		}
		return key, nil
	}
	if err != nil {
		return nil, err
	}
	var encoded string
	if err = json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("parse the sign-in key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) < 32 {
		return nil, errors.New("the sign-in key in the database is broken")
	}
	return key, nil
}

// ForgetFirstAdminPassword removes the file with a generated first password,
// once the admin has one of their own.
func ForgetFirstAdminPassword() { forgetFirstAdminPassword() }

// SignInKeepsAccounts reports whether the sign-in imported the accounts, so
// they change only through it and not behind its back.
func SignInKeepsAccounts() (bool, error) {
	var n int
	err := sqlDb.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'signin_docs'`).Scan(&n)
	if err != nil || n == 0 {
		return false, err
	}
	err = sqlDb.DB().QueryRow(`SELECT COUNT(*) FROM signin_docs`).Scan(&n)
	return n > 0, err
}
