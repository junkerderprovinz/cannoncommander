package state

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func isUserLookupNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err == errors.ErrNotExist || err.Error() == "user not found"
}

func noAuthAdminUsername() string {
	return settings.PasswordAdminUsername()
}

// ResolveNoAuthUser returns the user noauth mode impersonates at runtime: the password
// admin username when present, otherwise the sole user when exactly one account exists.
func ResolveNoAuthUser() (users.User, error) {
	target := noAuthAdminUsername()
	user, err := GetUserByUsername(target)
	if err == nil {
		return user, nil
	}
	if !isUserLookupNotFound(err) {
		return users.User{}, err
	}

	allUsers, err := GetAllUsers()
	if err != nil {
		return users.User{}, err
	}
	if len(allUsers) == 1 {
		return allUsers[0], nil
	}
	return users.User{}, errors.ErrNotExist
}
