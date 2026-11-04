package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func createTokenAuthUser(t *testing.T, username string, perms users.Permissions) *users.User {
	t.Helper()
	u := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := state.CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}
	stored, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	stored.Permissions = perms
	if err = state.UpdateUser(&stored, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	return &reloaded
}

// registerApiToken mints a named API token for user, stores its metadata, and
// registers its hash so auth can resolve it. configure, when non-nil, may adjust
// the metadata before it is persisted.
func registerApiToken(t *testing.T, user *users.User, name string, caps users.Permissions, minimal bool, configure func(*users.AuthToken)) string {
	t.Helper()
	tokenString, tokenMeta, err := auth.MakeSignedTokenAPI(user, name, time.Hour, caps, minimal)
	if err != nil {
		t.Fatal(err)
	}
	tokenMeta.Name = name
	tokenMeta.Token = tokenString
	if configure != nil {
		configure(&tokenMeta)
	}
	if err = state.AddUserToken(user.Username, tokenMeta); err != nil {
		t.Fatal(err)
	}
	if err = state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}
	return tokenString
}

func cookieRequest(t *testing.T, handler http.HandlerFunc, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "cannoncommander_session", Value: token})
	handler(rec, req)
	return rec
}

// A non-session bearer token whose hash has no stored permission metadata must be
// rejected rather than granted the owner's uncapped permissions.
func TestApiTokenWithoutMetadataRejected(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "metadata-less-admin", users.Permissions{Admin: true, Api: true})
	tokenString, _, err := auth.MakeSignedTokenAPI(user, "orphan-key", time.Hour, users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	// Register the hash mapping without any user.Tokens metadata.
	if err := state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("metadata-less API token: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestApiTokenMinimalKeepsOwnerPermissions(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "minimal-admin", users.Permissions{Admin: true, Api: true})
	tokenString := registerApiToken(t, user, "minimal-key", users.Permissions{}, true, nil)

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusOK {
		t.Fatalf("minimal API token: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestApiTokenCustomizedZeroPermissionsForbiddenOnAdmin(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "custom-zero-admin", users.Permissions{Admin: true, Api: true})
	tokenString := registerApiToken(t, user, "zero-key", users.Permissions{}, false, nil)

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("customized zero-cap API token: got status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestApiTokenLegacyEmptyCapsTreatedAsMinimal(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "legacy-minimal-admin", users.Permissions{Admin: true, Api: true})
	tokenString := registerApiToken(t, user, "legacy-key", users.Permissions{}, true, func(meta *users.AuthToken) {
		meta.Minimal = nil
	})

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy empty-cap API token: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// A named API token with stored caps must have those caps applied at auth time.
func TestApiTokenCapsAppliedFromStoredMetadata(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "capped-admin", users.Permissions{Admin: true, Api: true})

	// The token cap excludes admin, so the resolved user must lose admin.
	tokenString := registerApiToken(t, user, "capped-key", users.Permissions{Share: true}, false, nil)

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("capped API token: got status %d, want %d", rec.Code, http.StatusForbidden)
	}
}
