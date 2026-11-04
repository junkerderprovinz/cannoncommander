package toolaccess

import (
	"fmt"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// ErrEnforcedToolAccess is returned when a non-admin changes an enforced tool access value.
type ErrEnforcedToolAccess struct {
	ToolID users.ToolID
}

func (e ErrEnforcedToolAccess) Error() string {
	return fmt.Sprintf("tool access for %q is enforced by an administrator", e.ToolID)
}

// InitialToolAccessDefaultsDocument seeds all catalog tools as enabled and not enforced.
func InitialToolAccessDefaultsDocument() ToolAccessDefaultsDocument {
	items := make([]ToolAccessDefaultItem, 0, len(users.AllToolIDs))
	for _, id := range users.AllToolIDs {
		items = append(items, ToolAccessDefaultItem{
			ToolID:   id,
			Enabled:  true,
			Enforced: false,
		})
	}
	return ToolAccessDefaultsDocument{Items: items}
}

// EnsureCatalogInDefaults adds any catalog tools missing from the document.
func EnsureCatalogInDefaults(doc ToolAccessDefaultsDocument) (ToolAccessDefaultsDocument, bool) {
	if doc.Items == nil {
		doc.Items = []ToolAccessDefaultItem{}
	}
	present := make(map[users.ToolID]bool, len(doc.Items))
	for _, item := range doc.Items {
		if item.ToolID.Valid() {
			present[item.ToolID] = true
		}
	}
	changed := false
	for _, id := range users.AllToolIDs {
		if present[id] {
			continue
		}
		doc.Items = append(doc.Items, ToolAccessDefaultItem{
			ToolID:   id,
			Enabled:  true,
			Enforced: false,
		})
		changed = true
	}
	return doc, changed
}

// MergeDefaultToolAccess applies default enabled values for tools missing from u.ToolAccess.
func MergeDefaultToolAccess(u *users.User, doc ToolAccessDefaultsDocument) bool {
	if u == nil {
		return false
	}
	changed := false
	if u.ToolAccess == nil {
		u.ToolAccess = make(users.ToolAccessMap)
	}
	for _, item := range doc.Items {
		if _, exists := u.ToolAccess[item.ToolID]; exists {
			continue
		}
		u.ToolAccess[item.ToolID] = item.Enabled
		changed = true
	}
	return changed
}

// MergeEnforcedToolAccess overwrites enforced tool values for non-admin users.
func MergeEnforcedToolAccess(u *users.User, doc ToolAccessDefaultsDocument) bool {
	if u == nil || u.Permissions.Admin {
		return false
	}
	changed := false
	if u.ToolAccess == nil {
		u.ToolAccess = make(users.ToolAccessMap)
	}
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if u.ToolAccess[item.ToolID] != item.Enabled {
			u.ToolAccess[item.ToolID] = item.Enabled
			changed = true
		}
	}
	return changed
}

// ValidateEnforcedToolAccess returns an error if a non-admin changed an enforced tool value.
func ValidateEnforcedToolAccess(u *users.User, doc ToolAccessDefaultsDocument) error {
	if u == nil || u.Permissions.Admin {
		return nil
	}
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if u.ToolAccess == nil {
			return ErrEnforcedToolAccess{ToolID: item.ToolID}
		}
		if u.ToolAccess[item.ToolID] != item.Enabled {
			return ErrEnforcedToolAccess{ToolID: item.ToolID}
		}
	}
	return nil
}
