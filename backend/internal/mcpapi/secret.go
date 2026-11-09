package mcpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"io"
)

// Prefixes of everything the endpoint hands out. They make a leaked value
// recognisable to people and to secret scanners, and they let the gate tell an
// access token from a key before it looks anything up.
const (
	keyPrefix           = "ccmcp_"
	accessPrefix        = "ccmat_"
	refreshPrefix       = "ccmrt_"
	codePrefix          = "ccmac_"
	clientSecretPrefix  = "ccmcs_"
	clientIDPrefix      = "ccmci_"
	keyDomain           = "cannoncommander-mcp-key:"
	checkDomain         = "cannoncommander-mcp-check:"
	oauthDomain         = "cannoncommander-mcp-oauth-"
	consentDomain       = "cannoncommander-mcp-consent:"
	secretBytes         = 32
	encodedSecretLength = 43
)

// newSecret returns prefix followed by 32 random bytes in unpadded base64url.
func newSecret(prefix string) string {
	buf := make([]byte, secretBytes)
	// crypto/rand.Read never fails on the systems Go supports.
	_, _ = io.ReadFull(rand.Reader, buf)
	return prefix + base64.RawURLEncoding.EncodeToString(buf)
}

// newID returns a 32 hex character id for a key or a grant. The caller makes
// it because the check value is derived from it before the row exists.
func newID() string {
	buf := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, buf)
	return hex.EncodeToString(buf)
}

func mac(pepper []byte, msg string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// hashKey is the digest a key is stored and found by. A 256-bit random key
// needs no slow derivation, and the pepper means a copied database alone
// cannot confirm a guess.
func hashKey(pepper []byte, key string) string { return mac(pepper, keyDomain+key) }

// keyCheck is derived from the id alone, so a changed signing key can be told
// from a wrong key without the server ever seeing the key again.
func keyCheck(pepper []byte, id string) string { return mac(pepper, checkDomain+id) }

// hashOAuth separates the digests by kind, so an access token's digest can
// never stand in for a refresh token's.
func hashOAuth(pepper []byte, kind, value string) string {
	return mac(pepper, oauthDomain+kind+":"+value)
}

// keyHint is the last four characters of a key, which tell two keys apart in
// a list.
func keyHint(key string) string {
	if len(key) < 4 {
		return ""
	}
	return key[len(key)-4:]
}

func sameString(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// validVerifier reports whether v has the shape RFC 7636 section 4.1 gives a
// code verifier: 43 to 128 unreserved characters.
func validVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_', c == '~':
		default:
			return false
		}
	}
	return true
}

// pkceMatches reports whether verifier hashes to challenge under S256, the
// only method the server accepts.
func pkceMatches(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	return sameString(base64.RawURLEncoding.EncodeToString(sum[:]), challenge)
}

// validChallenge reports whether c has the shape of an S256 code challenge.
func validChallenge(c string) bool {
	if len(c) != encodedSecretLength {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(c)
	return err == nil
}
