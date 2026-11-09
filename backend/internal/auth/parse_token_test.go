package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func useSigningKey(t *testing.T, key string) {
	t.Helper()
	orig := settings.Config.Auth.Key
	settings.Config.Auth.Key = key
	t.Cleanup(func() { settings.Config.Auth.Key = orig })
}

func signHS256(t *testing.T, key string, claims jwt.RegisteredClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestParseTokenAcceptsAFreshToken(t *testing.T) {
	useSigningKey(t, "test-signing-key-parse")
	now := time.Now()
	raw := signHS256(t, "test-signing-key-parse", jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	})
	var claims jwt.RegisteredClaims
	token, err := ParseToken(raw, &claims)
	if err != nil || !token.Valid {
		t.Fatalf("fresh token: valid=%v err=%v", token != nil && token.Valid, err)
	}
}

func TestParseTokenRejectsTokensItCannotTrust(t *testing.T) {
	const key = "test-signing-key-parse"
	useSigningKey(t, key)
	now := time.Now()
	fresh := jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, fresh).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rs256, err := jwt.NewWithClaims(jwt.SigningMethodRS256, fresh).SignedString(rsaKey)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"expired", signHS256(t, key, jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		}), jwt.ErrTokenExpired},
		{"not valid yet", signHS256(t, key, jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
		}), jwt.ErrTokenNotValidYet},
		{"issued in the future", signHS256(t, key, jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
		}), jwt.ErrTokenUsedBeforeIssued},
		{"without expiry", signHS256(t, key, jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now)}), jwt.ErrTokenRequiredClaimMissing},
		{"signed with another key", signHS256(t, "some-other-key", fresh), jwt.ErrTokenSignatureInvalid},
		{"unsigned", unsigned, jwt.ErrTokenSignatureInvalid},
		{"signed with RS256", rs256, jwt.ErrTokenSignatureInvalid},
		{"empty", "", jwt.ErrTokenMalformed},
		{"two segments", "eyJhbGciOiJIUzI1NiJ9.e30", jwt.ErrTokenMalformed},
		{"not base64", "a.b.c", jwt.ErrTokenMalformed},
		{"payload not JSON", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.bm90LWpzb24.c2ln", jwt.ErrTokenMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var claims jwt.RegisteredClaims
			_, err := ParseToken(tc.raw, &claims)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseTokenRefusesEverythingWithoutASigningKey(t *testing.T) {
	useSigningKey(t, "")
	now := time.Now()
	raw := signHS256(t, "any-key", jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	})
	var claims jwt.RegisteredClaims
	if _, err := ParseToken(raw, &claims); !errors.Is(err, jwt.ErrTokenUnverifiable) {
		t.Fatalf("got %v, want %v", err, jwt.ErrTokenUnverifiable)
	}
}
