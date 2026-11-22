package state

import "github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"

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
