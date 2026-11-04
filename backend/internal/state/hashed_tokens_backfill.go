package state

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// BackfillHashedTokensFromUserRecords registers SHA256(raw JWT) → user_id for stored API tokens.
func BackfillHashedTokensFromUserRecords() error {
	if sqlDb == nil {
		return nil
	}
	list, err := sqlDb.ListUsers()
	if err != nil {
		return err
	}
	added := 0
	for _, user := range list {
		if user == nil || user.ID == 0 {
			continue
		}
		n, err := backfillUserTokenHashes(user)
		if err != nil {
			return fmt.Errorf("backfill hashed token for user %s: %w", user.Username, err)
		}
		added += n
	}
	if added > 0 {
		logger.Infof("Backfilled %d hashed token mapping(s) from user API token records", added)
	}
	return nil
}

// CollectStoredRawTokens returns deduplicated raw JWT strings from Tokens (Token
// field, falling back to legacy Key) and legacy ApiKeys.
func CollectStoredRawTokens(user *users.User) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(raw string) {
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	for _, tok := range user.Tokens {
		add(tok.Token)
		if tok.Token == "" {
			add(tok.Key)
		}
	}
	for _, tok := range user.ApiKeys {
		add(tok.Token)
		if tok.Token == "" {
			add(tok.Key)
		}
	}
	return out
}

// backfillUserTokenHashes maps each stored named API token to its owner. A
// persistence failure is returned so initialization aborts instead of leaving a
// mapping that silently stops working after restart. Legacy ApiKeys are
// included so pre-2.0.8 tokens without a hashed_tokens row work again.
// Existing rows are reconciled: tokens dead past the grace window have their
// mapping removed, and live tokens re-register so rows migrated before expiry
// tracking (expires_at=0) pick up the token's real expiry.
func backfillUserTokenHashes(user *users.User) (int, error) {
	if accessDb == nil || user == nil || user.ID == 0 {
		return 0, nil
	}
	added := 0
	now := time.Now()
	for _, raw := range CollectStoredRawTokens(user) {
		expiresAt := access.TokenExpiryUnix(raw)
		if access.TokenExpiredPastGrace(expiresAt, now) {
			if err := RemoveApiToken(raw); err != nil {
				return added, err
			}
			continue
		}
		if info, ok := accessDb.GetHashedTokenInfo(raw); ok && (expiresAt == 0 || info.ExpiresAt != 0) {
			// Already registered and either the token carries no expiry or the
			// stored expiry is already populated.
			continue
		}
		if err := AddApiToken(raw, user.ID); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
