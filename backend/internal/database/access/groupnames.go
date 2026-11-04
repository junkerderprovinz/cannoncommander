package access

import (
	"strings"
	"unicode"
)

// MaxGroupNameLength bounds names for manually created groups. Existing names
// (e.g. LDAP DNs synced from an IdP) may exceed it and are never rejected on read.
const MaxGroupNameLength = 128

// MinGroupNameLength bounds manually created group names. Existing records are
// never rejected so legacy single-character names stay usable.
const MinGroupNameLength = 2

// IsUsableSyncedGroupName reports whether a group name coming from an identity
// provider can be stored. Only the minimum needed to keep rows usable is
// rejected; everything else is kept so IdP naming conventions keep working.
func IsUsableSyncedGroupName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
