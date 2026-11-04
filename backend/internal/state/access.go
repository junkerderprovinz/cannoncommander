package state

import (
	"fmt"
	"slices"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
)

func GetUserGroups(username string) []string {
	return accessDb.GetUserGroups(username)
}

func AddUserToGroup(group, username string) error {
	return accessDb.AddUserToGroup(group, username)
}

func RemoveUserFromGroup(group, username string) error {
	return accessDb.RemoveUserFromGroup(group, username)
}

func SyncUserGroups(username string, newGroups []string) error {
	return accessDb.SyncUserGroups(username, newGroups)
}

// GroupExists reports whether a group name is already registered.
func GroupExists(groupname string) bool {
	if accessDb == nil {
		return false
	}
	return slices.Contains(accessDb.GetAllGroups(), groupname)
}

// RemoveUserFromAllGroups removes a username from every group's member list.
func RemoveUserFromAllGroups(username string) error {
	if accessDb == nil {
		return nil
	}
	return accessDb.RemoveUserFromAllGroups(username)
}

// RenameUserInGroups replaces one username with another in all group member lists.
func RenameUserInGroups(oldName, newName string) error {
	if accessDb == nil {
		return nil
	}
	return accessDb.RenameUserInGroups(oldName, newName)
}

// SetAccessSQLStoreForTest replaces the SQL persister used by access control (tests only).
func SetAccessSQLStoreForTest(store access.SQLPersister) {
	if accessDb != nil {
		accessDb.SetSQLStore(store)
	}
}

func AddApiToken(tokenString string, userID uint64) error {
	if accessDb == nil {
		return fmt.Errorf("access storage not available")
	}
	return accessDb.AddApiToken(tokenString, userID)
}

// BearerTokenGrace re-exports access.BearerTokenGrace for callers that should
// not depend on the access package directly.
const BearerTokenGrace = access.BearerTokenGrace

// RegisterSessionToken maps a session bearer JWT to its owner user id.
func RegisterSessionToken(tokenString string, userID uint64) error {
	if accessDb == nil {
		return fmt.Errorf("access storage not available")
	}
	return accessDb.AddSessionToken(tokenString, userID)
}

// HashedTokenOwner returns the owner id and session type for a registered bearer
// token hash. ok is false when the hash has no mapping (unknown or already revoked).
func HashedTokenOwner(tokenString string) (userID uint64, isSession bool, ok bool) {
	if accessDb == nil {
		return 0, false, false
	}
	info, found := accessDb.GetHashedTokenInfo(tokenString)
	if !found {
		return 0, false, false
	}
	return info.UserID, info.IsSession, true
}

// SessionAlive reports whether the session token with this hash still logs in.
func SessionAlive(tokenHash string) bool {
	return accessDb != nil && accessDb.SessionAlive(tokenHash)
}

// RevokeTokenHash ends the token with this hash at once.
func RevokeTokenHash(tokenHash string) error {
	if accessDb == nil {
		return fmt.Errorf("access storage not available")
	}
	return accessDb.RevokeTokenHash(tokenHash)
}

// RevokeSessions ends every session of the user but the one with the token
// hash keep and returns how many ended.
func RevokeSessions(userID uint64, keep string) (int, error) {
	if accessDb == nil {
		return 0, fmt.Errorf("access storage not available")
	}
	return accessDb.RevokeSessions(userID, keep)
}

func RemoveApiToken(tokenString string) error {
	return accessDb.RemoveApiToken(tokenString)
}

func IsTokenRevoked(token string) bool {
	if accessDb == nil {
		return false
	}
	return accessDb.IsTokenRevoked(token)
}

func RevokeToken(token string) error {
	return accessDb.RevokeToken(token)
}

// GetGroupMembers returns every group with its sorted member usernames.
func GetGroupMembers() map[string][]string {
	return accessDb.GetGroupMembers()
}

// SetGroupMembers creates the group if needed and replaces its member list.
func SetGroupMembers(group string, usernames []string) error {
	return accessDb.SetGroupMembers(group, usernames)
}

// DeleteGroup removes a group.
func DeleteGroup(group string) error {
	return accessDb.DeleteGroup(group)
}
