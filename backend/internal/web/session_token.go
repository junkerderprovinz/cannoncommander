package web

import (
	"fmt"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// mintAndRegisterSessionToken creates a minimal session JWT and registers its hash for auth lookup.
func mintAndRegisterSessionToken(user *users.User) (string, error) {
	return mintSessionToken(user, time.Hour*time.Duration(settings.Config.Auth.TokenExpirationHours))
}

// mintSessionToken is mintAndRegisterSessionToken for a session that lasts life.
func mintSessionToken(user *users.User, life time.Duration) (string, error) {
	if user == nil || user.ID == 0 {
		return "", fmt.Errorf("invalid user for session token")
	}
	name := "WEB_TOKEN_" + utils.InsecureRandomIdentifier(4)
	tokenString, _, err := auth.MakeSignedTokenAPI(user, name, life, user.Permissions, true)
	if err != nil {
		return "", err
	}
	if err := state.RegisterSessionToken(tokenString, user.ID); err != nil {
		return "", err
	}
	return tokenString, nil
}
