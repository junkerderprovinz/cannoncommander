package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5/request"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// ExtractToken prefers the Authorization header over the session cookie, so
// API clients can authenticate while a revoked browser cookie is present.
func ExtractToken(r *http.Request) (string, error) {
	hasToken := false

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

func sessionCookieDomain(r *http.Request) string {
	return strings.Split(requestHost(r), ":")[0]
}

const sessionCookieName = "cannoncommander_session"

// sessionCookie builds the JWT cookie. HttpOnly prevents srcdoc XSS from reading
// document.cookie. Secure is set when the request is HTTPS, or came in on the
// https address the config names, which a proxy with untrusted headers hides.
func sessionCookie(r *http.Request, token string, expiresTime time.Time, maxAge int, sameSite http.SameSite) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Domain:   sessionCookieDomain(r),
		Path:     "/",
		SameSite: sameSite,
		HttpOnly: true,
		Secure:   requestScheme(r) == "https" || onExternalHTTPS(r),
		Expires:  expiresTime,
		MaxAge:   maxAge,
	}
}

// onExternalHTTPS reports whether r came in on the host of an https ExternalUrl.
// Other hosts stay out, a browser would drop the cookie on a plain LAN address.
func onExternalHTTPS(r *http.Request) bool {
	u, err := url.Parse(settings.Config.Http.ExternalUrl)
	return err == nil && u.Scheme == "https" && strings.EqualFold(u.Hostname(), sessionCookieDomain(r))
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
