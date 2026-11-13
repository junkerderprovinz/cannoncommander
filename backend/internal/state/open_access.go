package state

import (
	"sync/atomic"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// nobodySignsIn starts false, so the server stays closed until the accounts
// were counted.
var nobodySignsIn atomic.Bool

// OpenAccess reports whether every request acts as the admin without a sign
// in: under noauth, or while no account here has a password or a passkey.
// Single sign on always asks, because people sign in there.
func OpenAccess() bool {
	m := settings.Config.Auth.Methods
	if m.NoAuth {
		return true
	}
	if m.OidcAuth.Enabled {
		return false
	}
	return nobodySignsIn.Load()
}

// SignsInHere reports whether u has a credential that signs in on this
// server.
func SignsInHere(u *users.User) bool {
	return u.LoginMethod == users.LoginMethodPassword && (u.Password != "" || len(u.PasskeyCredentials) > 0)
}

// recountSignIns runs after every change to accounts. A failed count keeps
// the instance closed, which locks nobody out who has a password.
func recountSignIns() {
	list, err := sqlDb.ListUsers()
	if err != nil {
		logger.Errorf("could not tell whether anyone can sign in: %v", err)
		nobodySignsIn.Store(false)
		return
	}
	for _, u := range list {
		if u != nil && SignsInHere(u) {
			nobodySignsIn.Store(false)
			return
		}
	}
	nobodySignsIn.Store(true)
}

// RemoveSignIn takes the password, the second factor and the passkeys off
// the account with id, so it signs in nowhere.
func RemoveSignIn(id uint64) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	stored, err := loadExistingUserLocked(&users.User{ID: id})
	if err != nil {
		return err
	}
	u := cloneUserPtr(stored)
	u.Password, u.TOTPSecret, u.TOTPNonce = "", "", ""
	u.OtpEnabled, u.RequirePasswordChange, u.PasskeyCredentials = false, false, nil
	if err := commitUserUpdate(u, stored); err != nil {
		return err
	}
	recountSignIns()
	return nil
}
