package mcpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
)

func init() {
	migrate.Register(migrate.Main, migrate.SQL("mcp/001_create",
		`CREATE TABLE mcp_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			oauth_client TEXT NOT NULL DEFAULT '',
			resource TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL,
			client TEXT NOT NULL DEFAULT '',
			key_digest TEXT NOT NULL DEFAULT '',
			key_hint TEXT NOT NULL DEFAULT '',
			key_check TEXT NOT NULL DEFAULT '',
			can_write INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			rotated_at INTEGER NOT NULL DEFAULT 0,
			last_used_at INTEGER NOT NULL DEFAULT 0,
			last_used_from TEXT NOT NULL DEFAULT '',
			revoked_at INTEGER NOT NULL DEFAULT 0,
			revoked_reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX mcp_keys_user ON mcp_keys (user_id, revoked_at)`,
		`CREATE UNIQUE INDEX mcp_keys_digest ON mcp_keys (key_digest) WHERE key_digest != ''`,
		`CREATE TABLE mcp_key_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_id TEXT NOT NULL,
			at INTEGER NOT NULL,
			tool TEXT NOT NULL,
			outcome TEXT NOT NULL,
			job_id TEXT NOT NULL DEFAULT '',
			routine INTEGER NOT NULL
		)`,
		`CREATE INDEX mcp_key_events_key ON mcp_key_events (key_id, routine, id)`,
		`CREATE TABLE mcp_key_calls (
			key_id TEXT NOT NULL,
			slot INTEGER NOT NULL,
			calls INTEGER NOT NULL,
			PRIMARY KEY (key_id, slot)
		)`,
		`CREATE TABLE mcp_oauth_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			enabled INTEGER NOT NULL,
			issuer TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE mcp_oauth_clients (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			redirect_uris TEXT NOT NULL,
			auth_method TEXT NOT NULL,
			secret_digest TEXT NOT NULL,
			known TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			created_from TEXT NOT NULL
		)`,
		`CREATE TABLE mcp_oauth_tokens (
			digest TEXT PRIMARY KEY,
			grant_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			spent_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX mcp_oauth_tokens_grant ON mcp_oauth_tokens (grant_id)`,
	))
}

// Key is one key an assistant authenticates with, or one client that signed
// in through OAuth, which is a Key of kind kindOAuth. The key itself is shown
// once and never stored: Digest is its peppered HMAC, Hint its last four
// characters, and Check a value derived from the id alone, which tells a
// changed signing key apart from a wrong key.
type Key struct {
	ID            string
	UserID        uint64
	Kind          string
	OAuthClient   string
	Resource      string
	Label         string
	Client        string
	Digest        string
	Hint          string
	Check         string
	CanWrite      bool
	CreatedAt     int64
	RotatedAt     int64
	LastUsedAt    int64
	LastUsedFrom  string
	RevokedAt     int64
	RevokedReason string
}

const (
	kindKey   = "key"
	kindOAuth = "oauth"
)

// keyLimit is how many keys one user may have active at once, and as many
// clients signed in through OAuth. Several named clients are the point; a list
// that grows past this is a sign nobody retires them.
const keyLimit = 10

var (
	errKeyNotFound   = errors.New("mcp key not found")
	errKeyLimit      = errors.New("mcp key limit reached")
	errKeyLabelTaken = errors.New("an active mcp key of this user already has this name")
	errKeyActive     = errors.New("mcp key is still active")
	errGrantLimit    = errors.New("oauth grant limit reached")
	errTokenNotFound = errors.New("oauth token not found")
	errRefreshReused = errors.New("a spent refresh token was presented again")
	errClientUnknown = errors.New("oauth client not found")
)

const keyCols = `id, user_id, kind, oauth_client, resource, label, client, key_digest, key_hint, key_check,
	can_write, created_at, rotated_at, last_used_at, last_used_from, revoked_at, revoked_reason`

type scanner interface{ Scan(...any) error }

// uid is a user id as the tables keep it. Ids are random 64-bit numbers, and
// SQLite holds no integer with the high bit set.
func uid(id uint64) string { return strconv.FormatUint(id, 10) }

func scanKey(s scanner) (Key, error) {
	var k Key
	var user string
	err := s.Scan(&k.ID, &user, &k.Kind, &k.OAuthClient, &k.Resource, &k.Label, &k.Client, &k.Digest, &k.Hint, &k.Check,
		&k.CanWrite, &k.CreatedAt, &k.RotatedAt, &k.LastUsedAt, &k.LastUsedFrom, &k.RevokedAt, &k.RevokedReason)
	if err != nil {
		return Key{}, err
	}
	k.UserID, err = strconv.ParseUint(user, 10, 64)
	return k, err
}

type store struct{ db *sql.DB }

// anyActiveKey reports whether some user has a key that can authenticate,
// which is what decides whether /mcp exists at all.
func (s store) anyActiveKey(ctx context.Context) (bool, error) {
	var found bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM mcp_keys
		WHERE kind = 'key' AND revoked_at = 0 AND key_digest != '')`).Scan(&found)
	return found, err
}

// keyByDigest returns the active key whose digest is digest.
func (s store) keyByDigest(ctx context.Context, digest string) (Key, error) {
	k, err := scanKey(s.db.QueryRowContext(ctx, `SELECT `+keyCols+` FROM mcp_keys
		WHERE key_digest = ? AND kind = 'key' AND revoked_at = 0`, digest))
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, errKeyNotFound
	}
	return k, err
}

// key returns one key, revoked or not.
func (s store) key(ctx context.Context, id string) (Key, error) {
	k, err := scanKey(s.db.QueryRowContext(ctx, `SELECT `+keyCols+` FROM mcp_keys WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, errKeyNotFound
	}
	return k, err
}

// keys returns the keys of one user, or of everybody for userID 0: the
// active ones oldest first, then the revoked ones, the most recent first.
func (s store) keys(ctx context.Context, userID uint64) ([]Key, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+keyCols+` FROM mcp_keys
		WHERE ? OR user_id = ?
		ORDER BY revoked_at != 0, CASE WHEN revoked_at = 0 THEN created_at ELSE 0 END, revoked_at DESC, id`, userID == 0, uid(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// labelTaken reports whether another active row of the user carries label,
// compared without regard to case.
func labelTaken(tx *sql.Tx, userID uint64, label, exceptID string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT count(*) FROM mcp_keys
		WHERE user_id = ? AND revoked_at = 0 AND lower(label) = lower(?) AND id != ?`, uid(userID), label, exceptID).Scan(&n)
	return n > 0, err
}

// createKey stores a new key. The id comes from the caller because the check
// value is derived from it before the row exists. Limit and name are settled
// in the transaction that writes the row, so two creates at once cannot both
// find room.
func (s store) createKey(ctx context.Context, k Key) (Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Key{}, err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_keys WHERE user_id = ? AND kind = 'key' AND revoked_at = 0`, uid(k.UserID)).Scan(&active); err != nil {
		return Key{}, err
	}
	if active >= keyLimit {
		return Key{}, errKeyLimit
	}
	if taken, err := labelTaken(tx, k.UserID, k.Label, ""); err != nil {
		return Key{}, err
	} else if taken {
		return Key{}, errKeyLabelTaken
	}
	if _, err := tx.Exec(`INSERT INTO mcp_keys (id, user_id, kind, label, client, key_digest, key_hint, key_check, can_write, created_at)
		VALUES (?, ?, 'key', ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, uid(k.UserID), k.Label, k.Client, k.Digest, k.Hint, k.Check, k.CanWrite, k.CreatedAt); err != nil {
		return Key{}, err
	}
	if err := tx.Commit(); err != nil {
		return Key{}, err
	}
	return s.key(ctx, k.ID)
}

// rotateKey gives an active key new material and keeps its id, name and
// permission. A client signed in through OAuth has no key to replace.
func (s store) rotateKey(ctx context.Context, id, digest, hint, check string, now int64) (Key, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE mcp_keys SET key_digest = ?, key_hint = ?, key_check = ?, rotated_at = ?
		WHERE id = ? AND revoked_at = 0 AND kind = 'key'`, digest, hint, check, now, id)
	if err != nil {
		return Key{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Key{}, errKeyNotFound
	}
	return s.key(ctx, id)
}

// updateKey changes the name of an active key, its write switch, or both. A
// nil argument leaves that column alone.
func (s store) updateKey(ctx context.Context, id string, label *string, canWrite *bool) (Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Key{}, err
	}
	defer tx.Rollback()
	k, err := scanKey(tx.QueryRow(`SELECT `+keyCols+` FROM mcp_keys WHERE id = ? AND revoked_at = 0`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, errKeyNotFound
	}
	if err != nil {
		return Key{}, err
	}
	if label != nil {
		if taken, err := labelTaken(tx, k.UserID, *label, id); err != nil {
			return Key{}, err
		} else if taken {
			return Key{}, errKeyLabelTaken
		}
		if _, err := tx.Exec(`UPDATE mcp_keys SET label = ? WHERE id = ?`, *label, id); err != nil {
			return Key{}, err
		}
	}
	if canWrite != nil {
		if _, err := tx.Exec(`UPDATE mcp_keys SET can_write = ? WHERE id = ?`, *canWrite, id); err != nil {
			return Key{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Key{}, err
	}
	return s.key(ctx, id)
}

// revokeKey empties a key's digest and keeps the row, so the jobs it started
// still name it. A revoked key never comes back, and the tokens of a revoked
// grant go in the same step.
func (s store) revokeKey(ctx context.Context, id, reason string, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	found, err := revokeTx(tx, id, reason, now)
	if err != nil {
		return err
	}
	if !found {
		return errKeyNotFound
	}
	return tx.Commit()
}

func revokeTx(tx *sql.Tx, id, reason string, now int64) (bool, error) {
	res, err := tx.Exec(`UPDATE mcp_keys SET key_digest = '', revoked_at = ?, revoked_reason = ?
		WHERE id = ? AND revoked_at = 0`, now, reason, id)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	_, err = tx.Exec(`DELETE FROM mcp_oauth_tokens WHERE grant_id = ?`, id)
	return err == nil, err
}

// revokeUser revokes the active rows of one kind a user has and returns them.
func (s store) revokeUser(ctx context.Context, userID uint64, kind, reason string, now int64) ([]Key, error) {
	return s.revokeWhere(ctx, `SELECT id FROM mcp_keys WHERE user_id = ? AND kind = ? AND revoked_at = 0`, reason, now, uid(userID), kind)
}

// revokeGrants revokes every active OAuth grant and returns them, keys
// untouched.
func (s store) revokeGrants(ctx context.Context, reason string, now int64) ([]Key, error) {
	return s.revokeWhere(ctx, `SELECT id FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0`, reason, now)
}

func (s store) revokeWhere(ctx context.Context, query, reason string, now int64, args ...any) ([]Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	ids, err := idsOf(tx, query, args...)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := revokeTx(tx, id, reason, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out := make([]Key, 0, len(ids))
	for _, id := range ids {
		k, err := s.key(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

func idsOf(tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// purgeKey removes a revoked key with its log for good.
func (s store) purgeKey(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var revokedAt int64
	err = tx.QueryRow(`SELECT revoked_at FROM mcp_keys WHERE id = ?`, id).Scan(&revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return errKeyNotFound
	}
	if err != nil {
		return err
	}
	if revokedAt == 0 {
		return errKeyActive
	}
	if err := purgeTx(tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

func purgeTx(tx *sql.Tx, id string) error {
	for _, q := range []string{
		`DELETE FROM mcp_key_events WHERE key_id = ?`,
		`DELETE FROM mcp_key_calls WHERE key_id = ?`,
		`DELETE FROM mcp_oauth_tokens WHERE grant_id = ?`,
		`DELETE FROM mcp_keys WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return nil
}

// forgetUser removes every key of a deleted user with everything that hangs
// on them.
func (s store) forgetUser(ctx context.Context, userID uint64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids, err := idsOf(tx, `SELECT id FROM mcp_keys WHERE user_id = ?`, uid(userID))
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := purgeTx(tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// touchKey records when and from where a key was last used.
func (s store) touchKey(ctx context.Context, id string, at int64, from string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mcp_keys SET last_used_at = ?, last_used_from = ? WHERE id = ? AND revoked_at = 0`, at, from, id)
	return err
}

// Event is one tool call of a key, or one of its requests the endpoint refused
// before any tool ran, which leaves Tool empty. Outcome is "ok" or the code of
// the refusal, JobID the job a write started. The arguments of a call are
// never kept.
type Event struct {
	At      int64  `json:"at"`
	Tool    string `json:"tool"`
	Outcome string `json:"outcome"`
	JobID   string `json:"jobId,omitempty"`
	routine bool
}

// How much of a key's log is kept: the newest eventsKept writes that went
// through and the newest routineKept of everything else, none older than
// eventsMaxAge. An assistant that reads or retries makes hundreds of calls,
// and one cap for both would push its writes out within minutes.
const (
	eventsKept   = 500
	routineKept  = 200
	eventsMaxAge = 30 * 24 * 60 * 60
)

// Calls are also counted in quarter-hour slots, which outlive the cap on the
// log, so "calls today" stays right for a busy key. callsCovered is how far
// back the slots reach, two days, which holds today in any time zone.
const (
	callSlot     = 15 * 60
	callsCovered = 2 * 24 * 60 * 60
)

func (s store) recordEvent(ctx context.Context, keyID string, e Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	kept := eventsKept
	if e.routine {
		kept = routineKept
	}
	if _, err := tx.Exec(`INSERT INTO mcp_key_events (key_id, at, tool, outcome, job_id, routine) VALUES (?, ?, ?, ?, ?, ?)`,
		keyID, e.At, e.Tool, e.Outcome, e.JobID, e.routine); err != nil {
		return err
	}
	if e.Tool != "" {
		if _, err := tx.Exec(`INSERT INTO mcp_key_calls (key_id, slot, calls) VALUES (?, ?, 1)
			ON CONFLICT (key_id, slot) DO UPDATE SET calls = calls + 1`, keyID, e.At-e.At%callSlot); err != nil {
			return err
		}
	}
	steps := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM mcp_key_events WHERE key_id = ? AND routine = ? AND id NOT IN (
			SELECT id FROM mcp_key_events WHERE key_id = ? AND routine = ? ORDER BY id DESC LIMIT ?)`,
			[]any{keyID, e.routine, keyID, e.routine, kept}},
		{`DELETE FROM mcp_key_events WHERE at < ?`, []any{e.At - eventsMaxAge}},
		{`DELETE FROM mcp_key_calls WHERE slot < ?`, []any{e.At - callsCovered}},
	}
	for _, st := range steps {
		if _, err := tx.Exec(st.q, st.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s store) events(ctx context.Context, keyID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT at, tool, outcome, job_id FROM mcp_key_events WHERE key_id = ? ORDER BY id DESC`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.At, &e.Tool, &e.Outcome, &e.JobID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// callsSince counts each key's tool calls in the slots that begin at or after
// since.
func (s store) callsSince(ctx context.Context, since int64) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key_id, sum(calls) FROM mcp_key_calls WHERE slot >= ? GROUP BY key_id`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// OAuthSettings is whether the MCP endpoint offers sign-in through OAuth and
// the public address it does so under, the issuer of every token.
type OAuthSettings struct {
	Enabled bool
	Issuer  string
}

func (s store) oauthSettings(ctx context.Context) (OAuthSettings, error) {
	var o OAuthSettings
	err := s.db.QueryRowContext(ctx, `SELECT enabled, issuer FROM mcp_oauth_settings WHERE id = 1`).Scan(&o.Enabled, &o.Issuer)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthSettings{}, nil
	}
	return o, err
}

func (s store) setOAuthSettings(ctx context.Context, o OAuthSettings, now int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mcp_oauth_settings (id, enabled, issuer, updated_at) VALUES (1, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET enabled = excluded.enabled, issuer = excluded.issuer, updated_at = excluded.updated_at`,
		o.Enabled, o.Issuer, now)
	return err
}

// Limits of the authorization server. Registration needs no credentials, so
// the clients nobody signed in with are capped and expire; a client with a
// grant stays as long as the grant's row does.
const (
	unusedClientLimit   = 100
	unusedClientTTL     = 24 * 60 * 60
	pendingClientGrace  = 10 * 60
	unusedClientCeiling = 1000

	// spentRefreshKept is how many rotated-out refresh tokens of a grant are
	// remembered to recognise one presented again.
	spentRefreshKept = 20
	// refreshRetryWindow is how many seconds the refresh token spent last may
	// come back as a retry, from a client whose answer got lost on the way.
	refreshRetryWindow = 30
)

const unusedClient = `NOT EXISTS (SELECT 1 FROM mcp_keys k WHERE k.oauth_client = mcp_oauth_clients.id)`

func (s store) createClient(ctx context.Context, c oauthClient, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient, now-unusedClientTTL); err != nil {
		return err
	}
	if err := evictUnused(tx, unusedClientLimit, now-pendingClientGrace); err != nil {
		return err
	}
	if err := evictUnused(tx, unusedClientCeiling, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO mcp_oauth_clients (id, name, redirect_uris, auth_method, secret_digest, known, created_at, created_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, c.ID, c.Name, c.redirectJSON(), c.AuthMethod, c.SecretDigest, c.Known, now, c.CreatedFrom); err != nil {
		return err
	}
	return tx.Commit()
}

// evictUnused deletes the oldest unused clients registered at or before
// createdBy until one more fits under limit.
func evictUnused(tx *sql.Tx, limit int, createdBy int64) error {
	var unused int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_oauth_clients WHERE ` + unusedClient).Scan(&unused); err != nil {
		return err
	}
	over := unused - limit + 1
	if over <= 0 {
		return nil
	}
	_, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE id IN (
		SELECT id FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient+` ORDER BY created_at, id LIMIT ?)`, createdBy, over)
	return err
}

func (s store) client(ctx context.Context, id string) (oauthClient, error) {
	var c oauthClient
	var uris string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, redirect_uris, auth_method, secret_digest, known, created_at, created_from
		FROM mcp_oauth_clients WHERE id = ?`, id).Scan(&c.ID, &c.Name, &uris, &c.AuthMethod, &c.SecretDigest, &c.Known, &c.CreatedAt, &c.CreatedFrom)
	if errors.Is(err, sql.ErrNoRows) {
		return oauthClient{}, errClientUnknown
	}
	if err != nil {
		return oauthClient{}, err
	}
	if err := c.readRedirects(uris); err != nil {
		return oauthClient{}, fmt.Errorf("client %s: %w", id, err)
	}
	return c, nil
}

// grant is what a new grant row is made of. The token digests are stored in
// the same transaction, so no prune can find the grant without a refresh
// token.
type grant struct {
	ID, Label, Client, OAuthClient, Resource, Check string
	UserID                                          uint64
	CanWrite                                        bool
	AccessDigest, RefreshDigest                     string
	AccessExpires, RefreshExpires                   int64
}

// createGrant stores the grant a client earned by signing in. An earlier
// active grant of the same client and user is revoked as replaced, because
// the client holds only the newest tokens anyway. The name is made unique
// among the user's active rows by a number behind it.
func (s store) createGrant(ctx context.Context, g grant, now int64) (Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Key{}, err
	}
	defer tx.Rollback()
	earlier, err := idsOf(tx, `SELECT id FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0 AND oauth_client = ? AND user_id = ?`, g.OAuthClient, uid(g.UserID))
	if err != nil {
		return Key{}, err
	}
	for _, id := range earlier {
		if _, err := revokeTx(tx, id, "replaced", now); err != nil {
			return Key{}, err
		}
	}
	var active int
	if err := tx.QueryRow(`SELECT count(*) FROM mcp_keys WHERE kind = 'oauth' AND revoked_at = 0 AND user_id = ?`, uid(g.UserID)).Scan(&active); err != nil {
		return Key{}, err
	}
	if active >= keyLimit {
		return Key{}, errGrantLimit
	}
	label, err := freeLabel(tx, g.UserID, g.Label)
	if err != nil {
		return Key{}, err
	}
	if _, err := tx.Exec(`INSERT INTO mcp_keys (id, user_id, kind, oauth_client, resource, label, client, key_check, can_write, created_at)
		VALUES (?, ?, 'oauth', ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, uid(g.UserID), g.OAuthClient, g.Resource, label, g.Client, g.Check, g.CanWrite, now); err != nil {
		return Key{}, err
	}
	if err := insertTokens(tx, g.ID, g.AccessDigest, g.RefreshDigest, g.AccessExpires, g.RefreshExpires, now); err != nil {
		return Key{}, err
	}
	if err := tx.Commit(); err != nil {
		return Key{}, err
	}
	return s.key(ctx, g.ID)
}

// freeLabel returns base, or base followed by the first number no active row
// of the user carries yet, cut so the result stays within 64 characters.
func freeLabel(tx *sql.Tx, userID uint64, base string) (string, error) {
	for n := 1; n < 1000; n++ {
		label := base
		if n > 1 {
			suffix := " " + strconv.Itoa(n)
			if keep := 64 - utf8.RuneCountInString(suffix); utf8.RuneCountInString(base) > keep {
				label = string([]rune(base)[:keep])
			}
			label += suffix
		}
		taken, err := labelTaken(tx, userID, label, "")
		if err != nil {
			return "", err
		}
		if !taken {
			return label, nil
		}
	}
	return "", errKeyLabelTaken
}

func insertTokens(tx *sql.Tx, grantID, access, refresh string, accessExp, refreshExp, now int64) error {
	_, err := tx.Exec(`INSERT INTO mcp_oauth_tokens (digest, grant_id, kind, created_at, expires_at)
		VALUES (?, ?, 'access', ?, ?), (?, ?, 'refresh', ?, ?)`,
		access, grantID, now, accessExp, refresh, grantID, now, refreshExp)
	return err
}

// accessGrant returns the active grant an unexpired access token belongs to.
func (s store) accessGrant(ctx context.Context, digest string, now int64) (Key, error) {
	k, err := scanKey(s.db.QueryRowContext(ctx, `SELECT `+keyCols+` FROM mcp_keys
		WHERE kind = 'oauth' AND revoked_at = 0 AND id = (
			SELECT grant_id FROM mcp_oauth_tokens WHERE digest = ? AND kind = 'access' AND expires_at > ?)`, digest, now))
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, errTokenNotFound
	}
	return k, err
}

// rotateRefresh trades a live refresh token of clientID for a new access and
// refresh token, as long as its grant was issued for resource. A spent one
// presented again revokes the grant and answers errRefreshReused with it,
// because somebody may hold a copy, unless it is the one spent last and back
// within refreshRetryWindow: that is a client whose answer got lost. The
// newest access token stays beside the new one, so a request already on its
// way still gets through.
func (s store) rotateRefresh(ctx context.Context, oldDigest, clientID, resource, access, refresh string, accessExp, refreshExp, now int64) (Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Key{}, err
	}
	defer tx.Rollback()
	var grantID string
	var expires, spent int64
	err = tx.QueryRow(`SELECT grant_id, expires_at, spent_at FROM mcp_oauth_tokens WHERE digest = ? AND kind = 'refresh'`, oldDigest).
		Scan(&grantID, &expires, &spent)
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, errTokenNotFound
	}
	if err != nil {
		return Key{}, err
	}
	g, err := scanKey(tx.QueryRow(`SELECT `+keyCols+` FROM mcp_keys WHERE id = ? AND kind = 'oauth' AND revoked_at = 0`, grantID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (g.OAuthClient != clientID || g.Resource != resource)) {
		return Key{}, errTokenNotFound
	}
	if err != nil {
		return Key{}, err
	}
	if spent != 0 {
		var last string
		if err := tx.QueryRow(`SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0
			ORDER BY spent_at DESC, rowid DESC LIMIT 1`, grantID).Scan(&last); err != nil {
			return Key{}, err
		}
		if last != oldDigest || now-spent > refreshRetryWindow {
			if _, err := revokeTx(tx, grantID, "refresh-reuse", now); err != nil {
				return Key{}, err
			}
			if err := tx.Commit(); err != nil {
				return Key{}, err
			}
			return g, errRefreshReused
		}
	}
	if expires <= now {
		return Key{}, errTokenNotFound
	}
	steps := []struct {
		q    string
		args []any
	}{
		{`UPDATE mcp_oauth_tokens SET spent_at = ? WHERE grant_id = ? AND kind = 'refresh' AND spent_at = 0`, []any{now, grantID}},
		{`DELETE FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'access' AND digest NOT IN (
			SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'access' ORDER BY created_at DESC, rowid DESC LIMIT 1)`,
			[]any{grantID, grantID}},
		{`DELETE FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0 AND digest NOT IN (
			SELECT digest FROM mcp_oauth_tokens WHERE grant_id = ? AND kind = 'refresh' AND spent_at != 0
			ORDER BY spent_at DESC, rowid DESC LIMIT ?)`, []any{grantID, grantID, spentRefreshKept}},
	}
	for _, st := range steps {
		if _, err := tx.Exec(st.q, st.args...); err != nil {
			return Key{}, err
		}
	}
	if err := insertTokens(tx, grantID, access, refresh, accessExp, refreshExp, now); err != nil {
		return Key{}, err
	}
	if err := tx.Commit(); err != nil {
		return Key{}, err
	}
	return g, nil
}

// revokeToken is token revocation by the client itself. An access token is
// dropped on its own; a refresh token ends the grant, which it returns. A
// token that is unknown or belongs to another client changes nothing and is
// no error, as RFC 7009 wants.
func (s store) revokeToken(ctx context.Context, digest, clientID string, now int64) (Key, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Key{}, err
	}
	defer tx.Rollback()
	var grantID, kind string
	err = tx.QueryRow(`SELECT grant_id, kind FROM mcp_oauth_tokens WHERE digest = ?`, digest).Scan(&grantID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return Key{}, nil
	}
	if err != nil {
		return Key{}, err
	}
	g, err := scanKey(tx.QueryRow(`SELECT `+keyCols+` FROM mcp_keys WHERE id = ? AND kind = 'oauth'`, grantID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && g.OAuthClient != clientID) {
		return Key{}, nil
	}
	if err != nil {
		return Key{}, err
	}
	if kind == "access" {
		g = Key{}
		_, err = tx.Exec(`DELETE FROM mcp_oauth_tokens WHERE digest = ?`, digest)
	} else {
		var found bool
		if found, err = revokeTx(tx, grantID, "client", now); !found {
			g = Key{}
		}
	}
	if err != nil {
		return Key{}, err
	}
	return g, tx.Commit()
}

// pruneOAuth drops expired tokens, revokes the grants that no longer hold a
// live refresh token, and removes unused clients past their day.
func (s store) pruneOAuth(ctx context.Context, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM mcp_oauth_tokens WHERE expires_at <= ?`, now); err != nil {
		return err
	}
	idle, err := idsOf(tx, `SELECT id FROM mcp_keys k WHERE k.kind = 'oauth' AND k.revoked_at = 0
		AND NOT EXISTS (SELECT 1 FROM mcp_oauth_tokens t
			WHERE t.grant_id = k.id AND t.kind = 'refresh' AND t.spent_at = 0 AND t.expires_at > ?)`, now)
	if err != nil {
		return err
	}
	for _, id := range idle {
		if _, err := revokeTx(tx, id, "expired", now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM mcp_oauth_clients WHERE created_at <= ? AND `+unusedClient, now-unusedClientTTL); err != nil {
		return err
	}
	return tx.Commit()
}
