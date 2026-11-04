package access

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

type StringSet map[string]struct{}

// GroupMap maps group names to a set of usernames.
type GroupMap map[string]StringSet

// HashedTokenInfo describes a registered bearer token owner and type.
type HashedTokenInfo struct {
	UserID    uint64
	IsSession bool
	// ExpiresAt is the token's exp claim as unix time; 0 means no expiry (e.g.
	// non-JWT strings or rows persisted before expiry tracking).
	ExpiresAt int64
}

// BearerTokenGrace is how long a bearer token may still resolve after session
// rotation or after its JWT exp (e.g. share-ACL identity). In-flight requests
// still carrying the previous cookie must not 401 during rotation.
const BearerTokenGrace = 2 * time.Minute

// TokenExpiryUnix extracts the exp claim of a bearer JWT without verifying the
// signature; the value is only used for expiry bookkeeping and tokens are
// verified on the auth path. Returns 0 for non-JWT strings or tokens without
// an expiry claim.
func TokenExpiryUnix(tokenString string) int64 {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenString, claims); err != nil {
		return 0
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0
	}
	return int64(exp)
}

// TokenExpiredPastGrace reports whether a token expiry is older than the
// resolution grace window. expiresAt == 0 (unknown/none) never expires.
func TokenExpiredPastGrace(expiresAt int64, now time.Time) bool {
	return expiresAt != 0 && now.After(time.Unix(expiresAt, 0).Add(BearerTokenGrace))
}

// Storage keeps the groups and the bearer token registry.
type Storage struct {
	mux           sync.RWMutex
	Groups        GroupMap                   // key: group name, value: set of usernames - in-memory authoritative state
	RevokedTokens map[string]int64           // token hash → revocation unix time (0 = immediate) - in-memory authoritative state
	HashedTokens  map[string]HashedTokenInfo // maps token hash → owner/type - in-memory authoritative state
	Users         *users.Storage             // Reference to users storage
	sqlStore      SQLPersister               // SQL store for persistence
}

// SQLPersister interface for SQL persistence operations
type SQLPersister interface {
	SaveGroup(name string, members StringSet) error
	DeleteGroup(name string) error
	SaveRevokedToken(tokenHash string, revokedAt int64) error
	PersistImmediateTokenRevocation(tokenHash string) error
	PersistTokenRetirement(tokenHash string, revokedAt int64, pruned []string) error
	DeleteRevokedToken(tokenHash string) error
	SaveHashedToken(tokenHash string, userID uint64, isSession bool, expiresAt int64) error
	DeleteHashedToken(tokenHash string) error
	DeleteHashedTokensByUserID(userID uint64) error
}

// SetSQLStore sets the SQL store for persistence operations
func (s *Storage) SetSQLStore(sqlStore SQLPersister) {
	s.sqlStore = sqlStore
}

// persistGroupSQLNL upserts or deletes one groups row to match in-memory state.
// Caller must hold s.mux. If the group is absent from s.Groups, it is deleted from SQL.
func (s *Storage) persistGroupSQLNL(groupname string) error {
	if s.sqlStore == nil {
		return nil
	}
	members, ok := s.Groups[groupname]
	if !ok {
		if err := s.sqlStore.DeleteGroup(groupname); err != nil {
			return fmt.Errorf("failed to delete group %q from sql: %w", groupname, err)
		}
		return nil
	}
	if err := s.sqlStore.SaveGroup(groupname, members); err != nil {
		return fmt.Errorf("failed to save group %q: %w", groupname, err)
	}
	return nil
}

// NewStorage creates a new Storage instance.
func NewStorage(usersStore *users.Storage) *Storage {
	var s = &Storage{
		Groups:        make(GroupMap),
		RevokedTokens: make(map[string]int64),
		HashedTokens:  make(map[string]HashedTokenInfo),
		Users:         usersStore,
	}
	return s
}

// AddUserToGroup adds a username to a group.
func (s *Storage) AddUserToGroup(group, username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	createdGroup := false
	if _, ok := s.Groups[group]; !ok {
		s.Groups[group] = make(StringSet)
		createdGroup = true
	}
	if _, ok := s.Groups[group][username]; ok {
		return nil
	}
	s.Groups[group][username] = struct{}{}
	if err := s.persistGroupSQLNL(group); err != nil {
		delete(s.Groups[group], username)
		if createdGroup {
			delete(s.Groups, group)
		}
		return err
	}
	return nil
}

// GetAllGroups returns all group names.
func (s *Storage) GetAllGroups() []string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	groups := make([]string, 0, len(s.Groups))
	for group := range s.Groups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

// GetGroupMembers returns every group with its sorted member usernames.
func (s *Storage) GetGroupMembers() map[string][]string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	result := make(map[string][]string, len(s.Groups))
	for group, members := range s.Groups {
		names := make([]string, 0, len(members))
		for name := range members {
			names = append(names, name)
		}
		sort.Strings(names)
		result[group] = names
	}
	return result
}

// SetGroupMembers creates the group if needed and replaces its membership.
func (s *Storage) SetGroupMembers(group string, usernames []string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	members := make(StringSet, len(usernames))
	for _, name := range usernames {
		if name != "" {
			members[name] = struct{}{}
		}
	}
	previous, existed := s.Groups[group]
	s.Groups[group] = members
	if err := s.persistGroupSQLNL(group); err != nil {
		// Keep memory and SQL consistent: undo the in-memory change.
		if existed {
			s.Groups[group] = previous
		} else {
			delete(s.Groups, group)
		}
		return err
	}
	return nil
}

// DeleteGroup removes a group. The rules that name it are the caller.s to
// clean up.
func (s *Storage) DeleteGroup(group string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	members, ok := s.Groups[group]
	if !ok {
		return nil
	}
	delete(s.Groups, group)
	if err := s.persistGroupSQLNL(group); err != nil {
		s.Groups[group] = members
		return err
	}
	return nil
}

// GetUserGroups returns all groups for a specific user.
func (s *Storage) GetUserGroups(username string) []string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	var groups []string
	for group, users := range s.Groups {
		if _, ok := users[username]; ok {
			groups = append(groups, group)
		}
	}
	return utils.NonNilSlice(groups)
}

// SyncUserGroups updates a user's group memberships.
// It removes the user from groups not in the new list and adds them to new ones.
// Changes are write-through to SQL when a sqlStore is configured.
func (s *Storage) SyncUserGroups(username string, newGroups []string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Create a set of new groups for efficient lookup
	newGroupsSet := make(StringSet, len(newGroups))
	for _, g := range newGroups {
		if !IsUsableSyncedGroupName(g) {
			continue
		}
		newGroupsSet[g] = struct{}{}
	}

	affected := make(map[string]struct{})
	for group, members := range s.Groups {
		_, userIsInGroup := members[username]
		_, groupIsInNewSet := newGroupsSet[group]
		if userIsInGroup && !groupIsInNewSet {
			affected[group] = struct{}{}
		}
	}
	for group := range newGroupsSet {
		if _, ok := s.Groups[group]; !ok {
			affected[group] = struct{}{}
			continue
		}
		if _, ok := s.Groups[group][username]; !ok {
			affected[group] = struct{}{}
		}
	}

	before := make(map[string]StringSet, len(affected))
	for group := range affected {
		before[group] = cloneStringSet(s.Groups[group])
	}

	for group, members := range s.Groups {
		_, userIsInGroup := members[username]
		_, groupIsInNewSet := newGroupsSet[group]
		if userIsInGroup && !groupIsInNewSet {
			delete(s.Groups[group], username)
		}
	}
	for group := range newGroupsSet {
		if _, ok := s.Groups[group]; !ok {
			s.Groups[group] = make(StringSet)
		}
		s.Groups[group][username] = struct{}{}
	}

	affectedGroups := slices.Sorted(maps.Keys(affected))
	var persisted []string
	for _, group := range affectedGroups {
		if err := s.persistGroupSQLNL(group); err != nil {
			for _, g := range persisted {
				if restoreErr := s.restoreGroupSQLNL(g, before[g]); restoreErr != nil {
					logger.Errorf("failed to restore group %q in sql after sync failure: %v", g, restoreErr)
				}
			}
			for g, snap := range before {
				if len(snap) == 0 {
					delete(s.Groups, g)
				} else {
					s.Groups[g] = snap
				}
			}
			return err
		}
		persisted = append(persisted, group)
	}
	return nil
}

func (s *Storage) restoreGroupSQLNL(groupname string, members StringSet) error {
	if s.sqlStore == nil {
		return nil
	}
	if len(members) == 0 {
		if err := s.sqlStore.DeleteGroup(groupname); err != nil {
			return fmt.Errorf("failed to delete group %q from sql: %w", groupname, err)
		}
		return nil
	}
	if err := s.sqlStore.SaveGroup(groupname, members); err != nil {
		return fmt.Errorf("failed to save group %q: %w", groupname, err)
	}
	return nil
}

func cloneStringSet(src StringSet) StringSet {
	if len(src) == 0 {
		return nil
	}
	dst := make(StringSet, len(src))
	maps.Copy(dst, src)
	return dst
}

// RemoveUserFromGroup removes a username from a group.
func (s *Storage) RemoveUserFromGroup(group, username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	members, ok := s.Groups[group]
	if !ok {
		return nil
	}
	if _, ok := members[username]; !ok {
		return nil
	}
	delete(members, username)
	if err := s.persistGroupSQLNL(group); err != nil {
		s.Groups[group][username] = struct{}{}
		return err
	}
	return nil
}

// RemoveUserFromAllGroups removes a username from every group's member list.
// Group rows themselves are kept, even when left empty. Used when a user is deleted.
func (s *Storage) RemoveUserFromAllGroups(username string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	affected := make(map[string]StringSet)
	for group, members := range s.Groups {
		if _, ok := members[username]; ok {
			affected[group] = cloneStringSet(members)
		}
	}
	if len(affected) == 0 {
		return nil
	}
	for group := range affected {
		delete(s.Groups[group], username)
	}
	var persisted []string
	for _, group := range slices.Sorted(maps.Keys(affected)) {
		if err := s.persistGroupSQLNL(group); err != nil {
			for _, g := range persisted {
				if restoreErr := s.restoreGroupSQLNL(g, affected[g]); restoreErr != nil {
					logger.Errorf("failed to restore group %q in sql after failure: %v", g, restoreErr)
				}
			}
			for g, snap := range affected {
				s.Groups[g] = snap
			}
			return err
		}
		persisted = append(persisted, group)
	}
	return nil
}

// RenameUserInGroups replaces one username with another in every group member list.
// Used when a user's username changes so memberships follow the rename.
func (s *Storage) RenameUserInGroups(oldName, newName string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if oldName == newName {
		return nil
	}
	affected := make(map[string]StringSet)
	for group, members := range s.Groups {
		if _, ok := members[oldName]; ok {
			affected[group] = cloneStringSet(members)
		}
	}
	if len(affected) == 0 {
		return nil
	}
	for group := range affected {
		delete(s.Groups[group], oldName)
		s.Groups[group][newName] = struct{}{}
	}
	var persisted []string
	for _, group := range slices.Sorted(maps.Keys(affected)) {
		if err := s.persistGroupSQLNL(group); err != nil {
			for _, g := range persisted {
				if restoreErr := s.restoreGroupSQLNL(g, affected[g]); restoreErr != nil {
					logger.Errorf("failed to restore group %q in sql after failure: %v", g, restoreErr)
				}
			}
			for g, snap := range affected {
				s.Groups[g] = snap
			}
			return err
		}
		persisted = append(persisted, group)
	}
	return nil
}

// RevokeToken immediately invalidates a token: the owner mapping is removed and
// the hash is persisted as revoked with timestamp 0 (no grace window).
func (s *Storage) RevokeToken(tokenString string) error {
	return s.RevokeTokenHash(utils.HashSHA256(tokenString))
}

// RevokeTokenHash is RevokeToken for callers that only kept the hash, such as
// the list of open sessions.
func (s *Storage) RevokeTokenHash(tokenHash string) error {
	var rollback tokenRevokeRollback
	s.mux.Lock()
	rollback.capture(s, tokenHash)
	s.RevokedTokens[tokenHash] = 0
	delete(s.HashedTokens, tokenHash)
	sqlStore := s.sqlStore
	s.mux.Unlock()

	if sqlStore == nil {
		return nil
	}
	if err := sqlStore.PersistImmediateTokenRevocation(tokenHash); err != nil {
		s.mux.Lock()
		rollback.apply(s)
		s.mux.Unlock()
		return err
	}
	return nil
}

// RevokeSessions ends every session of the user except the one whose token
// hash is keep, and returns how many it ended. API tokens keep working.
func (s *Storage) RevokeSessions(userID uint64, keep string) (int, error) {
	s.mux.RLock()
	var hashes []string
	for hash, info := range s.HashedTokens {
		if info.UserID == userID && info.IsSession && hash != keep {
			hashes = append(hashes, hash)
		}
	}
	s.mux.RUnlock()
	for i, hash := range hashes {
		if err := s.RevokeTokenHash(hash); err != nil {
			return i, err
		}
	}
	return len(hashes), nil
}

type tokenRevokeRollback struct {
	tokenHash  string
	hadHashed  bool
	hashed     HashedTokenInfo
	hadRevoked bool
	revokedAt  int64
}

func (rb *tokenRevokeRollback) capture(s *Storage, tokenHash string) {
	rb.tokenHash = tokenHash
	if info, ok := s.HashedTokens[tokenHash]; ok {
		rb.hadHashed = true
		rb.hashed = info
	}
	if at, ok := s.RevokedTokens[tokenHash]; ok {
		rb.hadRevoked = true
		rb.revokedAt = at
	}
}

func (rb *tokenRevokeRollback) apply(s *Storage) {
	if rb.hadHashed {
		s.HashedTokens[rb.tokenHash] = rb.hashed
	} else {
		delete(s.HashedTokens, rb.tokenHash)
	}
	if rb.hadRevoked {
		s.RevokedTokens[rb.tokenHash] = rb.revokedAt
	} else {
		delete(s.RevokedTokens, rb.tokenHash)
	}
}

type tokenRetireRollback struct {
	tokenHash       string
	hadTokenRevoked bool
	tokenRevokedAt  int64
	prunedRevoked   map[string]int64
	prunedHashed    map[string]HashedTokenInfo
}

// IsTokenRevoked reports whether a token is revoked. Retired session tokens are
// only considered revoked once the grace window has elapsed.
func (s *Storage) IsTokenRevoked(tokenString string) bool {
	tokenHash := utils.HashSHA256(tokenString)
	s.mux.RLock()
	revokedAt, exists := s.RevokedTokens[tokenHash]
	s.mux.RUnlock()
	if !exists {
		return false
	}
	if revokedAt == 0 {
		return true
	}
	return time.Since(time.Unix(revokedAt, 0)) >= BearerTokenGrace
}

// AddApiToken maps a named API token string hash to an owner user id.
func (s *Storage) AddApiToken(tokenString string, userID uint64) error {
	return s.addHashedToken(tokenString, userID, false)
}

// AddSessionToken maps a session bearer token string hash to an owner user id.
func (s *Storage) AddSessionToken(tokenString string, userID uint64) error {
	return s.addHashedToken(tokenString, userID, true)
}

func (s *Storage) addHashedToken(tokenString string, userID uint64, isSession bool) error {
	expiresAt := TokenExpiryUnix(tokenString)
	// Tokens already past the expiry grace window are never registered; expired
	// mappings are pruned at startup load instead of on each registration.
	if TokenExpiredPastGrace(expiresAt, time.Now()) {
		return nil
	}
	tokenHash := utils.HashSHA256(tokenString)
	s.mux.Lock()
	s.HashedTokens[tokenHash] = HashedTokenInfo{UserID: userID, IsSession: isSession, ExpiresAt: expiresAt}
	sqlStore := s.sqlStore
	s.mux.Unlock()
	if sqlStore != nil {
		return sqlStore.SaveHashedToken(tokenHash, userID, isSession, expiresAt)
	}
	return nil
}

// GetUserIDFromToken retrieves the owner user id for a given token string (memory read; SQL populated at startup).
func (s *Storage) GetUserIDFromToken(tokenString string) (uint64, bool) {
	info, ok := s.GetHashedTokenInfo(tokenString)
	return info.UserID, ok
}

// GetHashedTokenInfo retrieves owner/type metadata for a given token string.
// Tokens past their expiry grace window resolve as unknown.
func (s *Storage) GetHashedTokenInfo(tokenString string) (HashedTokenInfo, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	tokenHash := utils.HashSHA256(tokenString)
	info, exists := s.HashedTokens[tokenHash]
	if !exists {
		return info, false
	}
	if TokenExpiredPastGrace(info.ExpiresAt, time.Now()) {
		return HashedTokenInfo{}, false
	}
	return info, true
}

// SessionAlive reports whether the session token with this hash still logs in:
// it is registered as a session, not expired and not revoked. A retired token
// counts as alive while its grace window lasts.
func (s *Storage) SessionAlive(tokenHash string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	info, ok := s.HashedTokens[tokenHash]
	if !ok || !info.IsSession || TokenExpiredPastGrace(info.ExpiresAt, time.Now()) {
		return false
	}
	revokedAt, revoked := s.RevokedTokens[tokenHash]
	return !revoked || (revokedAt != 0 && time.Since(time.Unix(revokedAt, 0)) < BearerTokenGrace)
}

// GetRevokedTokens returns a copy of the revocation map (hash → revocation unix time).
func (s *Storage) GetRevokedTokens() map[string]int64 {
	s.mux.RLock()
	defer s.mux.RUnlock()
	result := make(map[string]int64, len(s.RevokedTokens))
	for k, v := range s.RevokedTokens {
		result[k] = v
	}
	return result
}

// RemoveApiToken removes a token hash mapping (used when deleting API keys).
func (s *Storage) RemoveApiToken(tokenString string) error {
	tokenHash := utils.HashSHA256(tokenString)
	s.mux.Lock()
	delete(s.HashedTokens, tokenHash)
	sqlStore := s.sqlStore
	s.mux.Unlock()
	if sqlStore != nil {
		return sqlStore.DeleteHashedToken(tokenHash)
	}
	return nil
}

// RemoveHashedTokensForUser removes every bearer-token hash → user id mapping for that owner.
// Call when deleting a user so recycled usernames never resolve leftover token hashes to a new id.
func (s *Storage) RemoveHashedTokensForUser(userID uint64) error {
	if userID == 0 {
		return nil
	}
	s.mux.Lock()
	for hash, info := range s.HashedTokens {
		if info.UserID == userID {
			delete(s.HashedTokens, hash)
		}
	}
	sqlStore := s.sqlStore
	s.mux.Unlock()
	if sqlStore != nil {
		return sqlStore.DeleteHashedTokensByUserID(userID)
	}
	return nil
}
