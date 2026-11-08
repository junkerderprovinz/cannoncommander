package web

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// sessionClaims are the claims of a session token. SignedIn is when the
// session began, which a renewed token carries on, and Stay marks a session
// that stays signed in.
type sessionClaims struct {
	jwt.RegisteredClaims
	SignedIn *jwt.NumericDate `json:"auth_time,omitempty"`
	Stay     bool             `json:"stay,omitempty"`
}

// mintAndRegisterSessionToken creates a minimal session JWT and registers its hash for auth lookup.
func mintAndRegisterSessionToken(user *users.User) (string, error) {
	return mintSessionToken(user, time.Hour*time.Duration(settings.Config.Auth.TokenExpirationHours))
}

// mintSessionToken is mintAndRegisterSessionToken for a session that lasts life.
func mintSessionToken(user *users.User, life time.Duration) (string, error) {
	return issueSession(user, life, sessionClaims{})
}

// issueSession signs and registers a session token that lasts life, with
// SignedIn and Stay taken from c. Without SignedIn the session begins now.
func issueSession(user *users.User, life time.Duration, c sessionClaims) (string, error) {
	if user == nil || user.ID == 0 {
		return "", fmt.Errorf("invalid user for session token")
	}
	id, err := utils.RandomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	key, err := auth.JWTSigningKeyBytes()
	if err != nil {
		return "", err
	}
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		ID:        id,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(life)),
		Issuer:    auth.TokenIssuer,
	}
	if c.SignedIn == nil {
		c.SignedIn = c.IssuedAt
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(key)
	if err != nil {
		return "", err
	}
	if err := state.RegisterSessionToken(token, user.ID); err != nil {
		return "", err
	}
	return token, nil
}
