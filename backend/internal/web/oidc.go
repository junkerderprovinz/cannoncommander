package web

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"golang.org/x/oauth2"
)

const (
	oidcStateCookieName    = "cannoncommander_oidc_state"
	oidcRedirectCookieName = "cannoncommander_oidc_redirect"
	oidcFlowCookieMaxAge   = 600 // seconds
)

func oidcFlowCookie(r *http.Request, name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   sessionCookieDomain(r),
		HttpOnly: true,
		Secure:   requestScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

func clearOidcFlowCookies(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, oidcFlowCookie(r, oidcStateCookieName, "", -1))
	http.SetCookie(w, oidcFlowCookie(r, oidcRedirectCookieName, "", -1))
}

// validateOidcCallbackState checks the OAuth state parameter against the HttpOnly cookie set at login.
func validateOidcCallbackState(r *http.Request, w http.ResponseWriter) (postLoginRedirect string, status int, err error) {
	stateParam := r.URL.Query().Get("state")
	stateCookie, stateErr := r.Cookie(oidcStateCookieName)
	redirectCookie, _ := r.Cookie(oidcRedirectCookieName)
	clearOidcFlowCookies(w, r)

	if stateParam == "" || stateErr != nil || stateCookie.Value == "" || stateParam != stateCookie.Value {
		return "", http.StatusBadRequest, fmt.Errorf("invalid OIDC state")
	}

	postLoginRedirect = settings.Config.Http.BaseURL
	if redirectCookie != nil && redirectCookie.Value != "" {
		if path, ok := SafeRelativeRedirectPath(redirectCookie.Value); ok {
			postLoginRedirect = path
		} else {
			logger.Warningf("Blocked OIDC post-login redirect cookie value: %s", redirectCookie.Value)
		}
	}
	return postLoginRedirect, 0, nil
}

// userInfo holds all claims dynamically, plus pre-parsed Groups.
type userInfo struct {
	Claims map[string]interface{}
	Groups []string
}

// userInfoUnmarshaller handles unmarshalling all claims dynamically,
// while optionally parsing a configurable groups claim.
type userInfoUnmarshaller struct {
	userInfo    *userInfo
	groupsClaim string
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (u *userInfoUnmarshaller) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract groups if configured (supports nested claims like "resource_access.cannoncommander.roles")
	if u.groupsClaim != "" {
		u.userInfo.Groups = extractGroupsFromOIDCClaims(raw, u.groupsClaim)
	}

	u.userInfo.Claims = raw
	return nil
}

// extractGroupsFromOIDCClaims extracts groups from OIDC claims supporting nested paths.
// Supports both direct claims (e.g., "groups") and nested claims using '.' separator (e.g., "resource_access.cannoncommander.roles")
func extractGroupsFromOIDCClaims(claims map[string]interface{}, groupsClaimField string) []string {
	var groups []string

	if groupsClaimField == "" {
		return groups
	}

	// Try to resolve nested path first (e.g., "custom.groups")
	groupsVal := resolveOIDCNestedClaim(claims, groupsClaimField)

	if groupsVal == nil {
		// If nested path didn't work, try direct lookup
		if val, ok := claims[groupsClaimField]; ok {
			groupsVal = val
		}
	}

	if groupsVal != nil {
		groups = parseOIDCGroupsValue(groupsVal)
	}

	return groups
}

// resolveOIDCNestedClaim resolves nested OIDC claims using '.' as a separator
func resolveOIDCNestedClaim(claims map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(claims)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// parseOIDCGroupsValue parses OIDC claim values into a string slice of groups
// Returns nil for missing/empty non-array values, []string{} for empty arrays
func parseOIDCGroupsValue(groupsVal interface{}) []string {
	var groups []string

	switch val := groupsVal.(type) {
	case map[string]interface{}:
		// Groups as map keys - return empty slice for empty map
		groups = slices.Collect(maps.Keys(val))
	case []interface{}:
		// Groups as array of interfaces - always initialize as empty slice for arrays
		groups = []string{}
		for _, g := range val {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, strings.TrimSpace(groupStr))
			}
		}
	case []string:
		// Groups as array of strings - always initialize as empty slice for arrays
		groups = val
		if groups == nil {
			groups = []string{}
		}
	case string:
		// Single group as string or comma-separated groups
		// Return nil for empty strings, parsed array otherwise
		if val != "" {
			parts := strings.Split(val, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			groups = parts
		}
		// else: groups stays nil for empty string
	default:
		// Try to unmarshal as JSON array
		if jsonBytes, err := json.Marshal(val); err == nil {
			var groupsArray []string
			if err := json.Unmarshal(jsonBytes, &groupsArray); err == nil {
				// Initialize as empty slice if array was parsed
				if groupsArray == nil {
					groups = []string{}
				} else {
					groups = groupsArray
				}
			}
		}
	}

	return groups
}

// OidcRedirectURL builds the OAuth redirect_uri from trusted forwarded headers and the request host.
func OidcRedirectURL(r *http.Request) string {
	host, scheme := shareURLParams(r)
	return fmt.Sprintf("%s://%s%sapi/auth/oidc/callback", scheme, host, settings.Config.Http.BaseURL)
}

// oidcLoginHandler initiates OIDC login.
func oidcLoginHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	if !oidcCfg.Enabled {
		return http.StatusForbidden, fmt.Errorf("oidc authentication is not enabled")
	}

	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		Endpoint:     oidcCfg.Provider.Endpoint(),
		RedirectURL:  OidcRedirectURL(r),
		Scopes:       strings.Fields(oidcCfg.Scopes),
	}

	state, err := utils.RandomHex(16)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("generate OIDC state: %w", err)
	}

	postLogin := settings.Config.Http.BaseURL
	if rawRedirect := r.URL.Query().Get("redirect"); rawRedirect != "" {
		if path, ok := SafeRelativeRedirectPath(rawRedirect); ok {
			postLogin = path
		} else {
			logger.Warningf("Blocked OIDC post-login redirect: %s", rawRedirect)
		}
	}

	http.SetCookie(w, oidcFlowCookie(r, oidcStateCookieName, state, oidcFlowCookieMaxAge))
	http.SetCookie(w, oidcFlowCookie(r, oidcRedirectCookieName, postLogin, oidcFlowCookieMaxAge))

	authURL := oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
	return 0, nil
}

// oidcCallbackHandler handles OIDC callback.
func oidcCallbackHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	ctx := r.Context()
	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	if oidcCfg.Provider == nil || oidcCfg.Verifier == nil {
		// Ensure Provider and Verifier are initialized on application startup
		// This check is good, keep it.
		logger.Error("OIDC provider or verifier not initialized.")
		return http.StatusInternalServerError, fmt.Errorf("OIDC provider or verifier not initialized")
	}
	// If disableVerifyTLS is true, create a custom HTTP client
	// and set it in the context for the OIDC provider.
	if oidcCfg.DisableVerifyTLS {
		// Create a custom transport with InsecureSkipVerify set to true
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		customClient := &http.Client{
			Transport: transport,
		}
		ctx = oidc.ClientContext(ctx, customClient)
	}
	code := r.URL.Query().Get("code")
	postLoginRedirect, status, stateErr := validateOidcCallbackState(r, w)
	if stateErr != nil {
		return status, stateErr
	}

	oauth2Config := &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		Endpoint:     oidcCfg.Provider.Endpoint(), // Use endpoint from discovered provider
		RedirectURL:  OidcRedirectURL(r),
		Scopes:       strings.Fields(oidcCfg.Scopes),
	}

	// Exchange the authorization code for tokens
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		logger.Errorf("failed to exchange token: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to exchange token: %v", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	// accessToken := token.AccessToken // Access token is needed for UserInfo, already in 'token'

	var userdata userInfo            // Declare userdata here to be populated by either source
	var idTokenClaims map[string]any // Verified ID-token claims preserved across UserInfo fallback
	claimsFromIDToken := false       // Flag to track if we successfully got claims from ID token
	loginUsername := ""              // Variable to hold the login username

	// Create custom unmarshaller for userInfo
	userInfoUnmarshaller := &userInfoUnmarshaller{
		userInfo:    &userdata,
		groupsClaim: oidcCfg.GroupsClaim,
	}

	// --- Attempt to process ID Token ---
	if ok && rawIDToken != "" {
		// Verify the ID token
		// This uses the verifier initialized with the provider's JWKS endpoint and client ID
		idToken, verify_err := oidcCfg.Verifier.Verify(ctx, rawIDToken)
		if verify_err != nil {
			// this might not be necessary for certain providers like authentik
			logger.Debugf("failed to verify ID token: %v. This might be expected, falling back to UserInfo endpoint.", verify_err)
			// Verification failed, claimsFromIDToken remains false
		} else {
			// Decode the ID token claims using custom unmarshaller
			// This is where the JWE unmarshalling error occurs if the token is encrypted
			if err := idToken.Claims(userInfoUnmarshaller); err != nil {
				logger.Warningf("failed to decode ID token claims: %v. Falling back to UserInfo endpoint.", err)
				// Claims decoding failed, claimsFromIDToken remains false
			} else {
				// Successfully verified and decoded ID token claims
				logger.Debugf("ID Token verified and claims decoded: %+v", userdata)
				if len(userdata.Claims) > 0 {
					idTokenClaims = maps.Clone(userdata.Claims)
				}

				// Decide if we rely on ID token claims or still need UserInfo
				// Even if parsing succeeded, if essential claims are missing, use UserInfo
				if _, ok := userdata.Claims[oidcCfg.UserIdentifier]; ok {
					claimsFromIDToken = true
					needsGroups := oidcCfg.AdminGroup != "" || len(oidcCfg.UserGroups) > 0
					if needsGroups && len(userdata.Groups) == 0 {
						logger.Debugf("OIDC: ID token missing groups claim (adminGroup/userGroups configured); fetching UserInfo")
						claimsFromIDToken = false
					}
				}
			}
		}

	} else {
		logger.Debug("No ID token found in token response or it was empty. Falling back to UserInfo endpoint.")
		// claimsFromIDToken remains false
	}

	// --- Fallback to UserInfo endpoint if ID token processing did not provide essential claims ---
	if !claimsFromIDToken {
		// Use the access token obtained from the initial exchange
		// oauth2Config.TokenSource creates a token source that uses the provided token.
		userInfoResp, err := oidcCfg.Provider.UserInfo(ctx, oauth2Config.TokenSource(ctx, token))
		if err != nil {
			logger.Errorf("failed to fetch user info from endpoint: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to fetch user info from endpoint: %v", err)
		}
		// Decode the UserInfo response using custom unmarshaller
		// The UserInfo endpoint is expected to return standard JSON
		if err := userInfoResp.Claims(userInfoUnmarshaller); err != nil {
			logger.Errorf("failed to decode user info from endpoint: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to decode user info from endpoint: %v", err)
		}
		if len(idTokenClaims) > 0 {
			mergeMissingOidcClaims(&userdata, idTokenClaims)
		}
	}

	// --- Determine login username dynamically ---
	loginUsername, err = oidcLoginName(userdata.Claims, oidcCfg.UserIdentifier)
	if errors.Is(err, errUnverifiedEmail) {
		logger.Warningf("OIDC sign in refused: %v", err)
		return http.StatusForbidden, err
	}
	if err != nil {
		logger.Errorf("No valid username found for identifier '%v' in claims.", oidcCfg.UserIdentifier)
		return http.StatusInternalServerError, err
	}

	// Proceed to log the user in with the OIDC data
	// userdata struct now contains info from either verified ID token or UserInfo endpoint
	// The issuer is the one in the settings, which the provider's discovery
	// and the ID token were checked against.
	subject, _ := userdata.Claims["sub"].(string)
	if subject == "" {
		logger.Errorf("the identity provider sent no subject for %s", loginUsername)
		return http.StatusInternalServerError, fmt.Errorf("the identity provider sent no subject")
	}
	id := oidcIdentity{issuer: oidcCfg.IssuerUrl, subject: subject, name: loginUsername, groups: userdata.Groups}
	return loginWithOidcUser(w, r, id, postLoginRedirect)
}

var errUnverifiedEmail = errors.New("the identity provider has not verified this email address")

// oidcLoginName takes the name of the account from the claims. An email nobody
// verified can be anybody's, since users can often type in their own, and it
// would lead straight into the account of the person it belongs to.
func oidcLoginName(claims map[string]any, identifier string) (string, error) {
	name, _ := claims[identifier].(string)
	if name == "" {
		return "", fmt.Errorf("no valid username found for identifier '%v'", identifier)
	}
	if verified, said := claims["email_verified"].(bool); identifier == "email" && said && !verified {
		return "", errUnverifiedEmail
	}
	return name, nil
}

// mergeMissingOidcClaims copies claims from from into userdata when absent after UserInfo fallback.
func mergeMissingOidcClaims(userdata *userInfo, from map[string]any) {
	if userdata.Claims == nil {
		userdata.Claims = make(map[string]any)
	}
	for k, v := range from {
		if _, exists := userdata.Claims[k]; !exists {
			userdata.Claims[k] = v
		}
	}
}

// loginWithOidcUser signs in the person id stands for, creating an account on
// the first sign in.
func loginWithOidcUser(w http.ResponseWriter, r *http.Request, id oidcIdentity, postLoginRedirect string) (int, error) {
	oidcCfg := settings.Config.Auth.Methods.OidcAuth
	db := state.MainDB()

	if len(oidcCfg.UserGroups) > 0 && !inAnyGroup(id.groups, oidcCfg.UserGroups) {
		logger.Warningf("User %s is not in any of the required groups %v. Access denied.", id.name, oidcCfg.UserGroups)
		endOidcAccess(r, db, id)
		return http.StatusForbidden, fmt.Errorf("user %s is not authorized to access this application (not in required groups)", id.name)
	}

	// The admin right follows the groups only when there is an admin group and
	// the provider sent groups. Otherwise admins are made in the app.
	var admin *bool
	if oidcCfg.AdminGroup != "" && id.groups != nil {
		inGroup := slices.Contains(id.groups, oidcCfg.AdminGroup)
		admin = &inGroup
	}

	account, err := oidcAccount(r.Context(), db, id, admin != nil && *admin)
	if errors.Is(err, errOidcNameTaken) {
		logger.Warningf("OIDC sign in refused: %s %s claims the name %s, which is bound to someone else", id.issuer, id.subject, id.name)
		return http.StatusConflict, err
	}
	if err == nil {
		err = syncOidcRights(&account, admin, id.groups)
	}
	if err != nil {
		if status, mapped := loginMethodHTTPStatus(err); status != 0 {
			return status, mapped
		}
		return http.StatusUnauthorized, err
	}
	user := &account
	username := user.Username
	logger.Debugf("Successfully authenticated OIDC username: %s", username)

	expires := time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)
	tokenString, err2 := mintAndRegisterSessionToken(user)
	if err2 != nil {
		// Handle potential errors during token generation
		if strings.Contains(err2.Error(), "key already exists with same name") {
			return http.StatusConflict, fmt.Errorf("token name conflict: %v", err2)
		}
		return http.StatusInternalServerError, fmt.Errorf("failed to generate authentication token for user %s: %v", username, err)
	}

	// Add 30 minutes buffer so expired token doesn't get automatically deleted by the browser
	// This allows backend to identify expired sessions and provide better user feedback
	expiresTime := time.Now().Add(expires).Add(time.Minute * 30)

	// Set the authentication token as an HTTP cookie (Lax for OIDC redirect return).
	http.SetCookie(w, sessionCookie(r, tokenString, expiresTime, 0, http.SameSiteLaxMode))

	// Set the authenticated user in the request context or response writer
	// This allows subsequent handlers to access the current user.
	SetUserInResponseWriter(w, user)

	activity.RecordLogin(r, user)

	fbRedirect := postLoginRedirect
	if fbRedirect == "" {
		fbRedirect = settings.Config.Http.BaseURL
	}
	if path, ok := SafeRelativeRedirectPath(fbRedirect); ok {
		fbRedirect = path
	} else if fbRedirect != settings.Config.Http.BaseURL {
		logger.Warningf("Blocked OIDC post-login redirect: %s", fbRedirect)
		fbRedirect = settings.Config.Http.BaseURL
	}

	http.Redirect(w, r, fbRedirect, http.StatusFound)

	// Return 0 to indicate that the response has been handled by the redirect
	return 0, nil
}
