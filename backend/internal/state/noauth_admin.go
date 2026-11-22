package state

import (
	"sync/atomic"

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

// openOwner finds the row of the account the sign-in counts as the owner.
var openOwner atomic.Pointer[func() (uint64, bool)]

// UseOwner connects the open server to the sign-in's owner.
func UseOwner(owner func() (uint64, bool)) { openOwner.Store(&owner) }

// ResolveNoAuthUser returns the user an open server acts as: the sign-in's
// owner, whose id a rename keeps, otherwise the password admin username,
// otherwise the sole account, otherwise the admin first by name.
func ResolveNoAuthUser() (users.User, error) {
	if owner := openOwner.Load(); owner != nil {
		if id, ok := (*owner)(); ok {
			if u, err := GetUserByID(id); err == nil {
				return u, nil
			}
		}
	}
	user, err := GetUserByUsername(noAuthAdminUsername())
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
	var first *users.User
	for i := range allUsers {
		if u := &allUsers[i]; u.Permissions.Admin && (first == nil || u.Username < first.Username) {
			first = u
		}
	}
	if first == nil {
		return users.User{}, errors.ErrNotExist
	}
	return *first, nil
}
