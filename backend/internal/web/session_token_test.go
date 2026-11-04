package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestSessionTokensRemainIndependent(t *testing.T) {
	setupTestEnv(t)
	origKey := settings.Config.Auth.Key
	origExp := settings.Config.Auth.TokenExpirationHours
	settings.Config.Auth.Key = "test-signing-key-for-independent-sessions"
	settings.Config.Auth.TokenExpirationHours = 2
	t.Cleanup(func() {
		settings.Config.Auth.Key = origKey
		settings.Config.Auth.TokenExpirationHours = origExp
	})

	user := createTokenAuthUser(t, "session-user", users.Permissions{Api: true})
	admin := createTokenAuthUser(t, "session-admin", users.Permissions{Admin: true, Api: true})
	owners := []*users.User{user, user, admin}
	tokens := make([]string, len(owners))
	for i, owner := range owners {
		var err error
		tokens[i], err = mintAndRegisterSessionToken(owner)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i, token := range tokens {
		if userID, isSession, ok := state.HashedTokenOwner(token); !ok || !isSession || userID != owners[i].ID {
			t.Fatalf("session %d lost its owner: userID=%d isSession=%v ok=%v", i, userID, isSession, ok)
		}
		want := http.StatusForbidden
		if owners[i].Permissions.Admin {
			want = http.StatusOK
		}
		if rec := cookieRequest(t, withAdmin(mockHandler), token); rec.Code != want {
			t.Fatalf("session %d: got status %d, want %d", i, rec.Code, want)
		}
	}

	if err := state.RevokeToken(tokens[0]); err != nil {
		t.Fatal(err)
	}
	for i, token := range tokens {
		want := http.StatusOK
		if i == 0 {
			want = http.StatusUnauthorized
		}
		if rec := cookieRequest(t, withUser(mockHandler), token); rec.Code != want {
			t.Fatalf("session %d after revocation: got status %d, want %d", i, rec.Code, want)
		}
	}
}

func TestSessionTokenWithoutIDRemainsValid(t *testing.T) {
	setupTestEnv(t)
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-legacy-session"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := createTokenAuthUser(t, "legacy-session-user", users.Permissions{Api: true})
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		Issuer:    auth.TokenIssuer,
	}).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.RegisterSessionToken(token, user.ID); err != nil {
		t.Fatal(err)
	}
	if rec := cookieRequest(t, withUser(mockHandler), token); rec.Code != http.StatusOK {
		t.Fatalf("legacy session: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// Regression test for #3006: a request that already carried the session cookie
// must not re-emit Set-Cookie. Otherwise an in-flight response carrying a
// just-rotated-out token reverts the browser jar to a token that dies when its
// retirement grace expires.
func TestWithUserDoesNotEchoCookieToken(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "echo-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})

	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("cookie-carried request must not re-emit Set-Cookie, got %v", setCookie)
	}
}

func TestWithUserLeavesTheCookieJarToSignIn(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "bearer-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("a bearer token ended up as the session cookie: %v", setCookie)
	}
}

func TestALinkWithATokenDoesNotSignTheVisitorIn(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "link-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users?auth="+token, http.NoBody)
	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("the link planted a session: %v", setCookie)
	}
}
