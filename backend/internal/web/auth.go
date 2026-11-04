package web

import (
	libError "errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4/request"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/files"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Prefer explicit ?auth= query and Authorization header over the session cookie so API
// clients can authenticate while a revoked browser cookie is present. ?auth= comes before
// Authorization so server callbacks (e.g. OnlyOffice) with both query session JWT and a
// third-party Bearer header resolve to our own token.
func ExtractToken(r *http.Request) (string, error) {
	hasToken := false

	auth := r.URL.Query().Get("auth")
	if auth != "" {
		hasToken = true
		if strings.Count(auth, ".") == 2 {
			return auth, nil
		}
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		hasToken = true
		parts := strings.Split(authHeader, " ")
		if len(parts) > 1 && strings.EqualFold(parts[0], "bearer") && strings.Count(parts[1], ".") == 2 {
			return parts[1], nil
		}
	}

	tokenObj, err := r.Cookie("cannoncommander_session")
	if err == nil {
		hasToken = true
		token := tokenObj.Value
		if token != "" && strings.Count(token, ".") == 2 {
			return token, nil
		}
	}

	if hasToken {
		return "", fmt.Errorf("invalid token provided")
	}

	return "", request.ErrNoTokenInRequest
}

// getOrCreateAuthenticatedUser is a common helper for retrieving or auto-creating users
// that sign in through OIDC.
// When groupsPresent is false the groups claim was omitted and existing memberships are left unchanged.
// When groupsPresent is true, groups (including empty) are synced into access-control GroupMap.
func getOrCreateAuthenticatedUser(username string, loginMethod users.LoginMethod, isAdmin bool, groups []string, groupsPresent bool) (*users.User, error) {
	if err := ensureUserInAllowedGroups(groups); err != nil {
		return nil, err
	}

	// Try to get existing user
	userValue, err := state.GetUserByUsername(username)
	if err != nil {
		if !libError.Is(err, errors.ErrNotExist) {
			return nil, err
		}
		// Auto-create user on first authentication
		user := users.User{
			FrontendUser: users.FrontendUser{
				LoginMethod: loginMethod,
				Username:    username,
			},
		}
		if isAdmin {
			user.Permissions.Admin = true
		}

		err = state.CreateUser(&user, "")
		if err != nil {
			return nil, err
		}
		if dirErr := files.MakeUserDirs(&user, true); dirErr != nil {
			logger.Error(dirErr.Error())
		}

		// Fetch the created user
		userValue, err = state.GetUserByUsername(username)
		if err != nil {
			return nil, err
		}
	}
	// Sync admin from current group membership when the auth source sent groups.
	if groupsPresent {
		if userValue.Permissions.Admin != isAdmin {
			userValue.Permissions.Admin = isAdmin
			err = state.UpdateUser(&userValue, "", "permissions")
			if err != nil {
				return nil, err
			}
		}
	}
	// Verify login method matches
	if userValue.LoginMethod != loginMethod {
		logger.Debugf("login rejected: user %q attempted %s login but account loginMethod is %q", username, loginMethod, userValue.LoginMethod)
		return nil, errors.ErrWrongLoginMethod
	}
	// Sync IdP groups into access-control GroupMap (write-through). Skip when the
	// token/response omitted groups so a missing claim cannot wipe memberships.
	if groupsPresent {
		if err := state.SyncUserGroups(username, groups); err != nil {
			return nil, fmt.Errorf("failed to sync user %s groups: %w", username, err)
		}
	}

	return &userValue, nil
}

func isAdminFromGroups(adminGroup string, groups []string) bool {
	return adminGroup != "" && slices.Contains(groups, adminGroup)
}

// ensureUserInAllowedGroups rejects login when userGroups is configured and none
// of the IdP groups are allowed. Must run before auto-create so denied users
// leave no account behind.
func ensureUserInAllowedGroups(groups []string) error {
	allowedGroups := settings.Config.Auth.Methods.OidcAuth.UserGroups
	if len(allowedGroups) == 0 {
		return nil
	}
	for _, userGroup := range groups {
		for _, allowedGroup := range allowedGroups {
			if strings.EqualFold(userGroup, allowedGroup) {
				return nil
			}
		}
	}
	return fmt.Errorf("user is not in allowed groups")
}

func sessionCookieDomain(r *http.Request) string {
	return strings.Split(requestHost(r), ":")[0]
}

const sessionCookieName = "cannoncommander_session"

// sessionCookie builds the JWT cookie. HttpOnly prevents srcdoc XSS from reading
// document.cookie; Secure is set when the request is HTTPS (including via proxy).
func sessionCookie(r *http.Request, token string, expiresTime time.Time, maxAge int, sameSite http.SameSite) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Domain:   sessionCookieDomain(r),
		Path:     "/",
		SameSite: sameSite,
		HttpOnly: true,
		Secure:   requestScheme(r) == "https",
		Expires:  expiresTime,
		MaxAge:   maxAge,
	}
}

// SetSessionCookie sets the authentication token as an HTTP cookie.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresTime time.Time) {
	http.SetCookie(w, sessionCookie(r, token, expiresTime, 0, http.SameSiteStrictMode))
}

// applyNamedApiTokenGlobalCaps intersects owner globals with stored caps for named custom API tokens.
// Session tokens never reach this function; they are resolved as IsSession and keep full owner globals.
func applyNamedApiTokenGlobalCaps(user *users.User, tokenName string) {
	if user == nil || user.Tokens == nil {
		return
	}
	stored, ok := user.Tokens[tokenName]
	if !ok {
		return
	}
	if users.IsMinimalApiToken(stored) {
		return
	}
	user.Permissions = users.IntersectGlobalPermissions(user.Permissions, stored.Permissions)
}
