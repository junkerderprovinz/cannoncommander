package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestMinimalSignedTokenCarriesOnlyStandardClaims(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "tokenuser"}}
	tokenString, _, err := MakeSignedTokenAPI(user, "sync", time.Hour*24*365*10, users.Permissions{}, true)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}
	assertMinimalJWTClaims(t, tokenString)
}

func TestCustomizedTokenMetadataRetainsPermissions(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "tokenuser"}}
	_, claim, err := MakeSignedTokenAPI(user, "custom", time.Hour*24*365, users.Permissions{Api: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Permissions.Api {
		t.Fatalf("expected api permission in returned metadata: %+v", claim.Permissions)
	}
	tokenString, _, err := MakeSignedTokenAPI(user, "custom2", time.Hour*24*365, users.Permissions{Api: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	assertMinimalJWTClaims(t, tokenString)
}

func assertMinimalJWTClaims(t *testing.T, tokenString string) {
	t.Helper()
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenString, claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	for _, key := range []string{"Permissions", "permissions", "belongsTo", "username", "name"} {
		if _, ok := claims[key]; ok {
			t.Fatalf("minimal token must not include %q claim; got %v", key, claims)
		}
	}
}
