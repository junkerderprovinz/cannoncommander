package settings

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

const SourceDefaultsConfigLockMessage = "some source access defaults were set in the config file and cannot be changed here until an admin removes them from the config file"

// BuiltinDefaultSourceFilePermissions is used when no source access defaults are configured.
func BuiltinDefaultSourceFilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   false,
		Create:   false,
		Delete:   false,
	}
}

// NormalizeSourceFilePermissions returns built-in defaults when all flags are unset.
func NormalizeSourceFilePermissions(p users.SourceFilePermissions) users.SourceFilePermissions {
	if p.IsUnset() {
		return BuiltinDefaultSourceFilePermissions()
	}
	return p
}

// ApplySourceAccessDefaultsToAllSources copies the same template onto every configured source.
func ApplySourceAccessDefaultsToAllSources(perms users.SourceFilePermissions) {
	p := users.MarkSourceFilePermissionsConfigured(NormalizeSourceFilePermissions(perms))
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		src.Config.DefaultPermissions = p
	}
}

// DefaultSourceFilePermissions returns the effective global source access defaults.
func DefaultSourceFilePermissions() users.SourceFilePermissions {
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		if !src.Config.DefaultPermissions.IsUnset() {
			return NormalizeSourceFilePermissions(src.Config.DefaultPermissions)
		}
	}
	return BuiltinDefaultSourceFilePermissions()
}

// OverlayConfigSourceDefaults applies config-locked permission flags onto stored defaults.
func OverlayConfigSourceDefaults(stored users.SourceFilePermissions) users.SourceFilePermissions {
	if len(Env.ConfigSourceDefaultPermissions) == 0 {
		return stored
	}
	stored = NormalizeSourceFilePermissions(stored)
	for flag, val := range Env.ConfigSourceDefaultPermissions {
		setSourcePermissionFlag(&stored, flag, val)
	}
	return users.MarkSourceFilePermissionsConfigured(stored)
}
