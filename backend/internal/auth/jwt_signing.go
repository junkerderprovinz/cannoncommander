package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// JWTSigningKeyBytes returns the HMAC key or an error if it is unset (empty key must never verify).
func JWTSigningKeyBytes() ([]byte, error) {
	key := settings.Config.Auth.Key
	if len(key) == 0 {
		return nil, fmt.Errorf("auth signing key is not configured")
	}
	return []byte(key), nil
}

// JWTSigningKeyFunc is the keyFunc for the session and API tokens this server issues.
func JWTSigningKeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return JWTSigningKeyBytes()
	}
}
