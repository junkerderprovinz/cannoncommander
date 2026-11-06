package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestASharePageComesWithoutTheApp(t *testing.T) {
	saved := assetFs
	t.Cleanup(func() { assetFs = saved })
	assetFs = fstest.MapFS{
		"index.html": {Data: []byte("<html>app</html>")},
		"share.html": {Data: []byte(`<html><script type="module" src="/assets/share-abc.js"></script></html>`)},
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /s/{slug}", sharePageHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/s/k3x9qa-urlaub", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "share-abc.js") {
		t.Fatalf("page: %d %s", rec.Code, rec.Body)
	}
	h := rec.Header()
	if !strings.Contains(h.Get("Content-Security-Policy"), "script-src 'self';") || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Cache-Control") != "no-cache" {
		t.Fatalf("headers %v", h)
	}

	assetFs = fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/s/k3x9qa-urlaub", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a build without the page: %d", rec.Code)
	}
}

func expiresIn(t *testing.T, token string) time.Duration {
	t.Helper()
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		t.Fatal(err)
	}
	return time.Until(claims.ExpiresAt.Time)
}

func TestASessionEndsWithTheBrowserUnlessKept(t *testing.T) {
	setupTestEnv(t)
	origKey, origExp := settings.Config.Auth.Key, settings.Config.Auth.TokenExpirationHours
	settings.Config.Auth.Key, settings.Config.Auth.TokenExpirationHours = "test-signing-key-for-kept-sessions", 2
	t.Cleanup(func() { settings.Config.Auth.Key, settings.Config.Auth.TokenExpirationHours = origKey, origExp })
	user := createTokenAuthUser(t, "kept-user", users.Permissions{Api: true})

	start := func(keep time.Duration) *http.Cookie {
		rec := httptest.NewRecorder()
		if err := (sessionLogin{}).Start(rec, httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", nil), user, keep); err != nil {
			t.Fatal(err)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
			t.Fatalf("cookies %v", cookies)
		}
		return cookies[0]
	}

	c := start(0)
	if !c.Expires.IsZero() || c.MaxAge != 0 {
		t.Fatalf("a browser session got an expiry: %v %d", c.Expires, c.MaxAge)
	}
	if left := expiresIn(t, c.Value); left < 11*time.Hour || left > 13*time.Hour {
		t.Fatalf("a browser session runs %v", left)
	}

	c = start(7 * 24 * time.Hour)
	if d := time.Until(c.Expires); d < 7*24*time.Hour-time.Minute || d > 7*24*time.Hour+time.Minute {
		t.Fatalf("a kept session ends in %v", d)
	}
	if left := expiresIn(t, c.Value); left < 7*24*time.Hour-time.Minute {
		t.Fatalf("the token of a kept session runs %v", left)
	}

	req := httptest.NewRequest("POST", "http://localhost/api/v1/auth/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	if err := (sessionLogin{}).Stop(rec, req); err != nil {
		t.Fatal(err)
	}
	if !state.IsTokenRevoked(c.Value) {
		t.Fatal("the session still works after signing out")
	}
	if gone := rec.Result().Cookies(); len(gone) != 1 || gone[0].MaxAge >= 0 {
		t.Fatalf("the cookie was not cleared: %v", gone)
	}
}
