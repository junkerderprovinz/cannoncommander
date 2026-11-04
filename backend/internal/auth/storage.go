package auth

import (
	"encoding/base64"

	"crypto/sha256"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"golang.org/x/crypto/pbkdf2"
)

// InitializeEncryption initializes the encryption key for TOTP from config
func InitializeEncryption() error {
	if settings.Config.Auth.TotpSecret != "" {
		key, err := base64.StdEncoding.DecodeString(settings.Config.Auth.TotpSecret)
		keyLen := len(key)

		if err == nil && (keyLen == 16 || keyLen == 24 || keyLen == 32) {
			// Use the user-provided key if it's valid
			encryptionKey = key
		} else {
			// Otherwise, derive the key from the provided secret string
			logger.Warningf("totpSecret is not a valid Base64 encoded key. Deriving a key from it. For better security, generate a secret with 'openssl rand -base64 32'.")
			salt := []byte{0xda, 0x90, 0x45, 0xc3, 0x06, 0xb5, 0x99, 0x9f, 0xb6, 0xae, 0xfc, 0x14, 0xef, 0x27, 0x6e, 0x6a}
			encryptionKey = pbkdf2.Key([]byte(settings.Config.Auth.TotpSecret), salt, 4096, 32, sha256.New)
		}
	}
	return nil
}
