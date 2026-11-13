package state

import (
	"os"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// configuredAdminPassword returns the admin password set in the config, or ""
// when it is blank or the placeholder "admin", so the first admin starts
// without one.
func configuredAdminPassword() string {
	cfg := settings.PasswordAdminPassword()
	if cfg == "admin" {
		return ""
	}
	return cfg
}

// firstAdminPasswordFile may hold a generated password of the first admin,
// which the container prints below its startup banner until the admin
// changes it.
var firstAdminPasswordFile string

func forgetFirstAdminPassword() {
	if err := os.Remove(firstAdminPasswordFile); err != nil && !os.IsNotExist(err) {
		logger.Warningf("could not remove %s: %v", firstAdminPasswordFile, err)
	}
}
