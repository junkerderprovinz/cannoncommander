package auth

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const base64URLAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func decodeJWTBase64URL(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// equivalentSpelling returns a different compact JWT string with the same decoded signature bytes.
func equivalentSpelling(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		panic("equivalentSpelling: invalid JWT")
	}
	header, payload, signature := parts[0], parts[1], parts[2]
	orig, err := decodeJWTBase64URL(signature)
	if err != nil {
		panic("equivalentSpelling: decode canonical signature: " + err.Error())
	}

	last := signature[len(signature)-1]
	for _, c := range base64URLAlphabet {
		if byte(c) == last {
			continue
		}
		trial := signature[:len(signature)-1] + string(c)
		dec, err := decodeJWTBase64URL(trial)
		if err != nil || !bytes.Equal(dec, orig) {
			continue
		}
		respelled := header + "." + payload + "." + trial
		if respelled != token {
			return respelled
		}
	}
	panic("equivalentSpelling: no alternate spelling found")
}

func invalidSignatureSpelling(token string) string {
	parts := strings.Split(token, ".")
	header, payload, signature := parts[0], parts[1], parts[2]
	idx := strings.IndexByte(base64URLAlphabet, signature[len(signature)-1])
	replacement := base64URLAlphabet[((idx&0b110000)+16)%64]
	return header + "." + payload + "." + signature[:len(signature)-1] + string(replacement)
}

func parseSessionToken(tokenString string) error {
	var tk users.AuthToken
	_, err := ParseToken(tokenString, &tk)
	return err
}

func parseLenient(tokenString string) error {
	var tk users.AuthToken
	_, err := jwt.ParseWithClaims(tokenString, &tk, JWTSigningKeyFunc())
	return err
}

func TestParseTokenRejectsEquivalentSpelling(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-strict-decode"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 1, FrontendUser: users.FrontendUser{Username: "alice"}}
	canonical, _, err := MakeSignedTokenAPI(user, "WEB_TOKEN_test", time.Hour, user.Permissions, false)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}

	if err := parseSessionToken(canonical); err != nil {
		t.Fatalf("canonical token should parse: %v", err)
	}

	respelled := equivalentSpelling(canonical)
	if err := parseSessionToken(respelled); err == nil {
		t.Fatal("an equivalent spelling should be rejected")
	}

	tampered := invalidSignatureSpelling(canonical)
	if err := parseSessionToken(tampered); err == nil {
		t.Fatal("tampered signature should be rejected")
	}
}

func TestEquivalentSpellingParsesWithoutStrictDecoding(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-strict-decode"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 1, FrontendUser: users.FrontendUser{Username: "alice"}}
	canonical, _, err := MakeSignedTokenAPI(user, "WEB_TOKEN_test", time.Hour, user.Permissions, false)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}
	respelled := equivalentSpelling(canonical)
	if err := parseLenient(respelled); err != nil {
		t.Fatalf("equivalent spelling should parse without strict decoding: %v", err)
	}
}

func TestParseTokenRejectsOtherHMACAlgorithms(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-methods"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	now := time.Now()
	claims := jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(settings.Config.Auth.Key))
		if err != nil {
			t.Fatalf("sign with %s: %v", method.Alg(), err)
		}
		if err := parseSessionToken(signed); err == nil {
			t.Errorf("a token signed with %s and the right key should be rejected", method.Alg())
		}
		if err := parseLenient(signed); err != nil {
			t.Errorf("%s token should parse without the method check: %v", method.Alg(), err)
		}
	}
}
