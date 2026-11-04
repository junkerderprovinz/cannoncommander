package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Constants for TOTP (Time-based One-Time Password) configuration.
const (
	// IssuerName is the name of the application or service.
	IssuerName = "CannonCommander"

	// TokenValidTime defines the total duration a token is considered valid.
	// Note: The actual validation window might be slightly longer depending on the period.
	TokenValidTime = time.Minute * 2

	// TOTPPeriod is the standard duration in seconds that a TOTP code is valid.
	TOTPPeriod uint = 30

	// TOTPSecretSize is the byte length of the shared secret.
	TOTPSecretSize uint = 20

	// TOTPDigits specifies the number of digits in the OTP code.
	TOTPDigits = otp.DigitsSix

	// TOTPAlgorithm is the hashing algorithm to use.
	TOTPAlgorithm = otp.AlgorithmSHA1
)

var (
	// TOTPSkew allows for a certain number of periods of clock drift.
	// We calculate it to best match the TokenValidTime.
	// (2 * Skew + 1) * Period >= TokenValidTime
	// For a 2-minute (120s) window with a 30s period, we need a Skew of 2.
	// (2*2 + 1) * 30s = 150s (2.5 minutes). This is the closest we can get.
	TOTPSkew      uint = uint(TokenValidTime.Seconds()) / (2 * uint(TOTPPeriod))
	encryptionKey []byte
	TotpCache     = cache.NewCache[string](5 * time.Minute)
)

// encryptSecret uses AES-GCM to encrypt a plaintext secret.
// It returns the base64-encoded ciphertext and nonce, or an error.
func encryptSecret(secret string, key []byte) (string, string, error) {
	if len(key) != 32 {
		return "", "", fmt.Errorf("invalid encryption key length: must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(secret), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

// decryptSecret uses AES-GCM to decrypt a ciphertext using its key and nonce.
// It returns the plaintext secret or an error.
func decryptSecret(b64Ciphertext, b64Nonce string) (string, error) {
	if len(encryptionKey) != 32 {
		return "", fmt.Errorf("invalid encryption key length: must be 32 bytes")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(b64Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(b64Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// This error often means the key is wrong or the data is corrupt
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}

	return string(plaintext), nil
}

// NewTotpKey creates a fresh TOTP key for username with the settings of this server.
func NewTotpKey(username string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      IssuerName,
		AccountName: username,
		Period:      TOTPPeriod,
		SecretSize:  TOTPSecretSize,
		Digits:      TOTPDigits,
		Algorithm:   TOTPAlgorithm,
	})
}

// SealTotpSecret turns a plain secret into the form stored on the user,
// encrypted when the server has a TOTP key.
func SealTotpSecret(secret string) (stored, nonce string, err error) {
	if settings.Config.Auth.TotpSecret == "" {
		return secret, "", nil
	}
	return encryptSecret(secret, encryptionKey)
}

// OpenTotpSecret returns the plain secret behind a stored one.
func OpenTotpSecret(stored, nonce string) (string, error) {
	if settings.Config.Auth.TotpSecret == "" {
		return stored, nil
	}
	if nonce == "" {
		return "", fmt.Errorf("secret is encrypted but nonce is missing")
	}
	return decryptSecret(stored, nonce)
}

// TotpStep returns the time step at which code is valid for secret, looking
// as far around now as the login does. Callers use the step to refuse a code
// that was already used.
func TotpStep(secret, code string, now time.Time) (int64, bool) {
	opts := totp.ValidateOpts{Period: TOTPPeriod, Digits: TOTPDigits, Algorithm: TOTPAlgorithm}
	period := int64(TOTPPeriod)
	for k := -int64(TOTPSkew); k <= int64(TOTPSkew); k++ {
		at := now.Add(time.Duration(k*period) * time.Second)
		want, err := totp.GenerateCodeCustom(secret, at, opts)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return at.Unix() / period, true
		}
	}
	return 0, false
}
