package web

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// respelled writes the last character of the signature another way that
// decodes to the same bytes. Only strict base64 decoding tells the two apart.
func respelled(t *testing.T, token string) string {
	t.Helper()
	cut := strings.LastIndexByte(token, '.') + 1
	sig := token[cut:]
	want, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		t.Fatal(err)
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, c := range alphabet {
		trial := sig[:len(sig)-1] + string(c)
		if trial == sig {
			continue
		}
		if got, err := base64.RawURLEncoding.DecodeString(trial); err == nil && bytes.Equal(got, want) {
			return token[:cut] + trial
		}
	}
	t.Fatal("the signature has no other spelling")
	return ""
}

// registerRawApiToken registers raw as an API token of u called name,
// whatever the token carries.
func registerRawApiToken(t *testing.T, u *users.User, name, raw string) {
	t.Helper()
	minimal := true
	if err := state.AddUserToken(u.Username, users.AuthToken{Name: name, Token: raw, Minimal: &minimal}); err != nil {
		t.Fatal(err)
	}
	if err := state.AddApiToken(raw, u.ID); err != nil {
		t.Fatal(err)
	}
}

// waysIn checks for each way a token signs in whether raw gets through.
func waysIn(t *testing.T) map[string]func(raw string) bool {
	bearer := func(raw string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+raw)
		return r
	}
	return map[string]func(string) bool{
		"session cookie": func(raw string) bool {
			return cookieRequest(t, withUser(mockHandler), raw).Code == http.StatusOK
		},
		"bearer header": func(raw string) bool {
			rec := httptest.NewRecorder()
			withUser(mockHandler)(rec, bearer(raw))
			return rec.Code == http.StatusOK
		},
		"api v1": func(raw string) bool {
			u, err := authenticateV1(httptest.NewRecorder(), bearer(raw))
			return err == nil && u != nil
		},
		"webdav": func(raw string) bool {
			_, err := davSignIn(httptest.NewRecorder(), davRequest("anna", raw))
			return err == nil
		},
	}
}

// Every token here is registered to the user, so nothing but the JWT checks
// can turn it away.
func TestEveryWayInTurnsAwayATokenThatFailsTheJWTChecks(t *testing.T) {
	u := davEnv(t)
	key := []byte(settings.Config.Auth.Key)
	now := time.Now()
	at := func(d time.Duration) *jwt.NumericDate { return jwt.NewNumericDate(now.Add(d)) }
	fresh := jwt.RegisteredClaims{Issuer: auth.TokenIssuer, IssuedAt: at(0), ExpiresAt: at(time.Hour)}
	signed := func(method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	good, _, err := auth.MakeSignedTokenAPI(u, "good", time.Hour, users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	registerRawApiToken(t, u, "good", good)
	bad := map[string]string{
		"expired":                 signed(jwt.SigningMethodHS256, key, jwt.RegisteredClaims{IssuedAt: at(-2 * time.Hour), ExpiresAt: at(-time.Minute)}),
		"issued in the future":    signed(jwt.SigningMethodHS256, key, jwt.RegisteredClaims{IssuedAt: at(time.Hour), ExpiresAt: at(2 * time.Hour)}),
		"not valid yet":           signed(jwt.SigningMethodHS256, key, jwt.RegisteredClaims{IssuedAt: at(0), NotBefore: at(time.Hour), ExpiresAt: at(2 * time.Hour)}),
		"without expiry":          signed(jwt.SigningMethodHS256, key, jwt.RegisteredClaims{IssuedAt: at(0)}),
		"unsigned":                signed(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, fresh),
		"signed with RS256":       signed(jwt.SigningMethodRS256, rsaKey, fresh),
		"signed with HS512":       signed(jwt.SigningMethodHS512, key, fresh),
		"signed with another key": signed(jwt.SigningMethodHS256, []byte("some-other-key"), fresh),
		"spelled another way":     respelled(t, good),
	}
	for name, raw := range bad {
		registerRawApiToken(t, u, name, raw)
	}

	for way, signsIn := range waysIn(t) {
		if !signsIn(good) {
			t.Fatalf("%s: the good token did not sign in", way)
		}
		for name, raw := range bad {
			if signsIn(raw) {
				t.Errorf("%s: a token %s signed in", way, name)
			}
		}
	}
}

func TestARevokedTokenStaysOutInAnyOtherSpelling(t *testing.T) {
	u := davEnv(t)
	raw := registerApiToken(t, u, "rclone", users.Permissions{}, true, nil)
	ways := waysIn(t)
	for way, signsIn := range ways {
		if !signsIn(raw) {
			t.Fatalf("%s: the token did not sign in before it was revoked", way)
		}
	}
	if err := state.RevokeToken(raw); err != nil {
		t.Fatal(err)
	}
	other := respelled(t, raw)
	for way, signsIn := range ways {
		if signsIn(raw) || signsIn(other) {
			t.Errorf("%s: the revoked token signed in", way)
		}
	}
}

func TestAnExpiredSessionIsReportedAsExpired(t *testing.T) {
	u := davEnv(t)
	now := time.Now()
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
	}).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.RegisterSessionToken(raw, u.ID); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: raw})
	status, err := withUserHelper(nil)(httptest.NewRecorder(), r, &requestContext{Ctx: r.Context()})
	if status != http.StatusUnauthorized || !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("got %d, %v; want %d and an expired token", status, err, http.StatusUnauthorized)
	}
}
