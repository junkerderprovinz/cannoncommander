//go:generate go run ./tools/yaml.go -input=pkg/settings/settings.go -output=config.generated.yaml
package settings

import (
	"crypto/rand"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

const DefaultUsersHomeBasePath = "/users"

// AuthMethod describes an authentication method.
type AuthMethod string

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GetSettingsConfig(nameType string, Value string) string {
	return nameType + Value
}

func AdminPerms() users.Permissions {
	return users.Permissions{
		Share:    true,
		Admin:    true,
		Api:      true,
		Realtime: false,
	}
}

// AdminSourceFilePermissions returns full per-source file permissions for admin users.
func AdminSourceFilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   true,
		Delete:   true,
		Create:   true,
	}
}

// MergeDefaultEnabledBackendScopes ensures every defaultEnabled source is present in
// BackendScopes. Existing scopes for configured sources are preserved (empty Scope is
// filled from DefaultUserScope). Unknown paths not in Config.Server.Sources are kept
// at the end. defaultEnabled: false sources are never auto-added.
// declined lists defaultEnabled source paths the user explicitly removed via scope edits.
func MergeDefaultEnabledBackendScopes(existing []users.BackendScope, declined []string) []users.BackendScope {
	declinedSet := make(map[string]struct{}, len(declined))
	for _, path := range declined {
		declinedSet[path] = struct{}{}
	}
	byPath := make(map[string]users.BackendScope, len(existing))
	for _, s := range existing {
		byPath[s.Path] = s
	}

	newScopes := make([]users.BackendScope, 0, len(existing)+len(Config.Server.Sources))
	seen := make(map[string]struct{}, len(Config.Server.Sources))
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		existingScope, ok := byPath[src.Path]
		if ok {
			if existingScope.Scope == "" {
				existingScope.Scope = src.Config.DefaultUserScope
			}
		} else if src.Config.DefaultEnabled {
			if _, omitted := declinedSet[src.Path]; omitted {
				continue
			}
			existingScope = users.BackendScope{
				Scope: src.Config.DefaultUserScope,
			}
		} else {
			continue
		}
		newScopes = append(newScopes, users.BackendScope{
			Path:        src.Path,
			Scope:       existingScope.Scope,
			Permissions: existingScope.Permissions,
		})
		seen[src.Path] = struct{}{}
	}
	for _, s := range existing {
		if _, ok := seen[s.Path]; !ok {
			newScopes = append(newScopes, s)
		}
	}
	return newScopes
}

// ComputeDeclinedDefaultSources records defaultEnabled sources intentionally omitted
// from a user's persisted BackendScopes after an explicit scope edit.
func ComputeDeclinedDefaultSources(scopes []users.BackendScope) []string {
	present := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		present[scope.Path] = struct{}{}
	}
	var declined []string
	for _, src := range Config.Server.Sources {
		if src == nil || !src.Config.DefaultEnabled {
			continue
		}
		if _, ok := present[src.Path]; !ok {
			declined = append(declined, src.Path)
		}
	}
	return declined
}

// ApplySourceDefaults gives a new user the sources marked defaultEnabled and
// the default file permissions on every scope that has none yet.
func ApplySourceDefaults(u *users.User) {
	sourceDefaults := DefaultSourceFilePermissions()
	u.BackendScopes = MergeDefaultEnabledBackendScopes(u.BackendScopes, nil)
	if u.BackendSourcePermissions == nil {
		u.BackendSourcePermissions = make(map[string]users.SourceFilePermissions)
	}
	for i, scope := range u.BackendScopes {
		if _, ok := u.BackendSourcePermissions[scope.Path]; !ok {
			perms := sourceDefaults
			if u.Permissions.Admin {
				perms = AdminSourceFilePermissions()
			}
			u.BackendScopes[i].Permissions = perms
			u.BackendSourcePermissions[scope.Path] = perms
		} else if u.BackendScopes[i].Permissions.IsUnset() {
			u.BackendScopes[i].Permissions = u.BackendSourcePermissions[scope.Path]
		}
	}
	if u.LoginMethod == "" {
		u.LoginMethod = users.LoginMethodPassword
	}
	u.Version = users.CurrentUserMigrationVersion
}
