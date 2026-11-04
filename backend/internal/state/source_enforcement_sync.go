package state

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// ApplyEnforcedSourcePermissionsSyncToUser applies enforced source permission defaults onto u in memory.
// Returns true if scope permissions changed.
func ApplyEnforcedSourcePermissionsSyncToUser(u *users.User) bool {
	if u == nil {
		return false
	}
	defaults := GetSourceAccessDefaults()
	enforced := GetEnforcedSourcePermissions()
	return settings.SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced)
}
