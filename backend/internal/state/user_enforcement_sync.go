package state

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// ApplyEnforcedSyncToUser applies enforced default values onto u in memory. Returns true if profile fields changed.
func ApplyEnforcedSyncToUser(u *users.User) bool {
	if u == nil {
		return false
	}
	d := EffectiveUserDefaults()
	e := EffectiveEnforced()
	return settings.SyncEnforcedDefaultsOntoUser(u, d, e)
}
