package auth

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// MethodNoAuth is used to identify no auth.
const MethodNoAuth = "noauth"

// AuthenticateNoAuth authenticates as the configured admin user with no credentials required.
func AuthenticateNoAuth(r *http.Request, user *users.Storage) (*users.User, error) {
	admin := settings.PasswordAdminUsername()
	id, err := users.ResolveUsernameToID(admin)
	if err != nil {
		return nil, err
	}
	return user.Get(id)
}
