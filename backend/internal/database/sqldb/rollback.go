package sqldb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// The sign-in lives in its own documents from schema version 8 on. The
// users, tokens and codes it imported from stay for one release, so the
// version before can still run on this file. That version writes the
// schema version back down on its start, which is the tripwire, and the
// fingerprint over what it could have changed tells whether it did.

const legacyFingerprintKey = "signin.legacyFingerprint"

// legacyTables are the tables of the accounts package the older version
// signs people in with. They come from its migrations, so a store opened
// without that package has none of them.
var legacyTables = []string{"recovery_codes", "oidc_identities"}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func tokenDigest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// OlderBuildOpened reports whether a version before 8 opened the file since
// this one last ran on it.
func (s *SQLStore) OlderBuildOpened() bool { return s.olderBuild }

// StayTripped writes the schema version back below this one, so a start
// that refused to run on what the older version changed refuses again
// next time.
func (s *SQLStore) StayTripped() error {
	_, err := s.db.Exec(`UPDATE schema_version SET version = ?`, currentSchemaVersion-1)
	return err
}

// ArmLegacyFingerprint takes the fingerprint of the old sign-in tables as
// they are now. From then on every write of this version to them takes it
// again in the same transaction.
func (s *SQLStore) ArmLegacyFingerprint() error {
	s.armed.Store(true)
	return s.InLegacyTx(func(*sql.Tx) error { return nil })
}

// LegacyFingerprint returns the fingerprint taken last and the one of the
// tables now.
func (s *SQLStore) LegacyFingerprint() (stored, current string, err error) {
	raw, err := s.GetSetting(legacyFingerprintKey)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return "", "", err
	}
	if raw != nil {
		if err = json.Unmarshal(raw, &stored); err != nil {
			return "", "", err
		}
	}
	current, err = legacyFingerprint(s.db)
	return stored, current, err
}

// InLegacyTx runs fn in a transaction that also takes the fingerprint again
// once it is armed, for writes to the old sign-in tables.
func (s *SQLStore) InLegacyTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if s.armed.Load() {
		sum, err := legacyFingerprint(tx)
		if err != nil {
			return fmt.Errorf("fingerprint of the old sign-in: %w", err)
		}
		if err := s.saveSettingInTx(tx, legacyFingerprintKey, sum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func hasFingerprint(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = ?`, legacyFingerprintKey).Scan(&n)
	return n > 0, err
}

type legacyUser struct {
	id, name, data string
	api, admin, rt bool
}

// legacyFingerprint hashes everything the older version decides sign-in
// with and can change itself: accounts by id with name, method, rights and
// scopes, password, second factor, passkeys and API tokens with whether
// they were revoked, recovery codes and OIDC bindings. Sessions and when
// tokens were used change with every visit and stay out.
func legacyFingerprint(q querier) (string, error) {
	rows, err := q.Query(`SELECT user_id, username, perm_api, perm_admin, perm_realtime, user_data FROM users ORDER BY user_id`)
	if err != nil {
		return "", err
	}
	var all []legacyUser
	for rows.Next() {
		var u legacyUser
		if err = rows.Scan(&u.id, &u.name, &u.api, &u.admin, &u.rt, &u.data); err != nil {
			rows.Close()
			return "", err
		}
		all = append(all, u)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return "", err
	}

	h := sha256.New()
	field := func(v ...any) {
		for _, x := range v {
			fmt.Fprintf(h, "%q;", fmt.Sprint(x))
		}
		h.Write([]byte{'\n'})
	}
	for _, row := range all {
		var u users.User
		if err = finishUserLoad(&u, []byte(row.data)); err != nil {
			return "", fmt.Errorf("user %s: %w", row.id, err)
		}
		field("user", row.id, row.name, row.api, row.admin, row.rt, u.Permissions.Share, u.LoginMethod, u.Password, u.TOTPSecret, u.OtpEnabled)
		scopes, err := json.Marshal(u.BackendScopes)
		if err != nil {
			return "", err
		}
		field("scopes", string(scopes))
		var passkeys []string
		for _, c := range u.PasskeyCredentials {
			passkeys = append(passkeys, c.ID)
		}
		slices.Sort(passkeys)
		field("passkeys", passkeys)
		names := make([]string, 0, len(u.Tokens))
		for name := range u.Tokens {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			digest := tokenDigest(u.Tokens[name].Token)
			var revoked int
			err = q.QueryRow(`SELECT COUNT(*) FROM revoked_tokens WHERE token_hash = ?`, digest).Scan(&revoked)
			if err != nil {
				return "", err
			}
			field("token", name, digest, revoked > 0)
		}
	}
	for _, table := range legacyTables {
		if err := hashTable(q, table, field); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashTable(q querier, table string, field func(...any)) error {
	var n int
	if err := q.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	rows, err := q.Query(`SELECT * FROM ` + table + ` ORDER BY 1, 2, 3`)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		out := []any{table}
		for _, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			out = append(out, v)
		}
		field(out...)
	}
	return rows.Err()
}
