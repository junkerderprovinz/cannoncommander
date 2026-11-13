package state

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestConfiguredAdminPasswordPrefersTheNestedSetting(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = "from-nested"
	settings.Config.Auth.AdminPassword = "from-legacy"
	if got := configuredAdminPassword(); got != "from-nested" {
		t.Fatalf("password = %q, want from-nested", got)
	}
}

func TestConfiguredAdminPasswordFallsBackToTheTopLevelSetting(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	settings.Config.Auth.AdminPassword = "from-legacy"
	if got := configuredAdminPassword(); got != "from-legacy" {
		t.Fatalf("password = %q, want from-legacy", got)
	}
}

func TestBlankOrPlaceholderAdminPasswordMeansNone(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	for _, cfg := range []string{"", "admin"} {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = cfg
		settings.Config.Auth.AdminPassword = cfg
		if got := configuredAdminPassword(); got != "" {
			t.Fatalf("cfg=%q: password = %q, want none", cfg, got)
		}
	}
}
