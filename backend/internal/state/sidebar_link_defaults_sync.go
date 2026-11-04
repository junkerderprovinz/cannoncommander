package state

import (
	"fmt"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/usersidebar"
)

// ResyncSidebarLinkDefaultsForAllUsers merges enabled default sidebar links for every user.
func ResyncSidebarLinkDefaultsForAllUsers() error {
	return resyncSidebarLinkDefaultsForAllUsers(EffectiveSidebarLinkDefaults())
}

func resyncSidebarLinkDefaultsForAllUsers(doc usersidebar.SidebarLinkDefaultsDocument) error {
	if errInjectResyncSidebarDefaults != nil {
		return errInjectResyncSidebarDefaults
	}
	usersMux.Lock()
	defer usersMux.Unlock()

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for sidebar defaults sync: %w", err)
	}

	var updated int
	for _, row := range usersList {
		if row == nil {
			continue
		}
		u := cloneUserPtr(row)
		links, changed := usersidebar.MergeDefaultLinks(u.SidebarLinks, u.BackendScopes, doc)
		if !changed {
			continue
		}
		u.SidebarLinks = links
		if normalized, normChanged := usersidebar.NormalizeSidebarLinks(u.SidebarLinks); normChanged {
			u.SidebarLinks = normalized
		}
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync sidebar defaults for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced sidebar link defaults for %d users", updated)
	}
	return nil
}
