package sqldb

import (
	"database/sql"
	"fmt"
)

// currentSchemaVersion is the SQLite schema marker for this codebase.
const currentSchemaVersion = 7

// Schema creates all tables for the SQLite database
func createSchema(db *sql.DB) error {
	schema := `
	-- Schema version tracking
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		updated_at INTEGER NOT NULL
	);

	-- Users: user_id is the primary key (stable identity); username is unique (one active login per name)
	CREATE TABLE IF NOT EXISTS users (
		user_id TEXT PRIMARY KEY NOT NULL,
		username TEXT NOT NULL UNIQUE,
		perm_api INTEGER NOT NULL DEFAULT 0,
		perm_admin INTEGER NOT NULL DEFAULT 0,
		perm_realtime INTEGER NOT NULL DEFAULT 0,
		user_data TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_users_admin ON users(perm_admin);
	CREATE INDEX IF NOT EXISTS idx_users_api ON users(perm_api);

	-- Groups table
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		members TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);

	-- Revoked tokens table
	CREATE TABLE IF NOT EXISTS revoked_tokens (
		token_hash TEXT PRIMARY KEY,
		revoked_at INTEGER NOT NULL
	);

	-- Hashed tokens (minimal JWT → owner user_id)
	CREATE TABLE IF NOT EXISTS hashed_tokens (
		token_hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		is_session INTEGER NOT NULL DEFAULT 0,
		expires_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_hashed_tokens_user_id ON hashed_tokens(user_id);

	-- Index info table
	CREATE TABLE IF NOT EXISTS index_info (
		path TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		complexity INTEGER NOT NULL DEFAULT 0,
		num_dirs INTEGER NOT NULL DEFAULT 0,
		num_files INTEGER NOT NULL DEFAULT 0,
		scanners TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_index_info_source ON index_info(source);

	-- Auth methods table
	CREATE TABLE IF NOT EXISTS auth_methods (
		type TEXT PRIMARY KEY,
		config TEXT NOT NULL
	);

	-- Settings table
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	-- Activity audit log (append-only; purged by retention policy)
	CREATE TABLE IF NOT EXISTS activity_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		source TEXT,
		path TEXT,
		target_path TEXT,
		ip_address TEXT,
		status INTEGER NOT NULL,
		success INTEGER NOT NULL DEFAULT 1,
		details TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_activity_created_at ON activity_log(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_activity_user_created ON activity_log(user_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_activity_event_created ON activity_log(event_type, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_activity_user_event_created ON activity_log(user_id, event_type, created_at DESC);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// initializeSchemaVersion sets the initial schema version
func initializeSchemaVersion(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	if count == 0 {
		_, err = db.Exec("INSERT INTO schema_version (version, updated_at) VALUES (?, ?)",
			currentSchemaVersion, currentTimestamp())
		if err != nil {
			return fmt.Errorf("failed to initialize schema version: %w", err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version from the database
func getSchemaVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get schema version: %w", err)
	}
	return version, nil
}

func runMigrations(db *sql.DB, fromVersion int) error {
	if fromVersion > currentSchemaVersion {
		_, err := db.Exec("UPDATE schema_version SET version = ?, updated_at = ?",
			currentSchemaVersion, currentTimestamp())
		if err != nil {
			return fmt.Errorf("failed to normalize schema version: %w", err)
		}
		return nil
	}
	if fromVersion >= currentSchemaVersion {
		return nil
	}

	for v := fromVersion + 1; v <= currentSchemaVersion; v++ {
		switch v {
		case 1:
			// Canonical schema is createSchema; no step migrations.
		case 2:
			// Added the quota tables, which step 6 drops again.
		case 3:
			if err := addHashedTokenSessionColumn(db); err != nil {
				return err
			}
		case 4:
			if err := addHashedTokenExpiryColumn(db); err != nil {
				return err
			}
		case 5:
			if err := dropOldShareLinks(db); err != nil {
				return err
			}
		case 6:
			if _, err := db.Exec(`DROP TABLE IF EXISTS quotas; DROP TABLE IF EXISTS quota_counters`); err != nil {
				return fmt.Errorf("drop quotas: %w", err)
			}
		case 7:
			if _, err := db.Exec(`DELETE FROM settings WHERE key LIKE 'userDefaults.%' OR key IN ('sidebarLinkDefaults', 'toolAccessDefaults')`); err != nil {
				return fmt.Errorf("drop user defaults: %w", err)
			}
		default:
			return fmt.Errorf("unknown schema version: %d", v)
		}
	}

	_, err := db.Exec("UPDATE schema_version SET version = ?, updated_at = ?",
		currentSchemaVersion, currentTimestamp())
	if err != nil {
		return fmt.Errorf("failed to update schema version: %w", err)
	}

	return nil
}

// addHashedTokenSessionColumn adds the is_session flag to pre-existing
// hashed_tokens tables. Existing rows are treated as non-session (API) tokens.
func addHashedTokenSessionColumn(db *sql.DB) error {
	hasColumn, err := tableHasColumn(db, "hashed_tokens", "is_session")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE hashed_tokens ADD COLUMN is_session INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("failed to add hashed_tokens.is_session: %w", err)
	}
	return nil
}

// addHashedTokenExpiryColumn adds the expires_at timestamp to pre-existing
// hashed_tokens tables. Existing rows get 0 (unknown expiry) and are left
// untouched; expiry is recorded for newly registered tokens.
func addHashedTokenExpiryColumn(db *sql.DB) error {
	hasColumn, err := tableHasColumn(db, "hashed_tokens", "expires_at")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE hashed_tokens ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("failed to add hashed_tokens.expires_at: %w", err)
	}
	return nil
}

// tableHasColumn reports whether table/column exists in the current schema.
func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, fmt.Errorf("failed to inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("failed to scan table_info(%s): %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error iterating table_info(%s): %w", table, err)
	}
	return false, nil
}

// dropOldShareLinks removes the share links of the older interface. The
// links live in share_links, which the shares package keeps.
func dropOldShareLinks(db *sql.DB) error {
	if _, err := db.Exec(`DROP TABLE IF EXISTS shares; DELETE FROM settings WHERE key LIKE 'shareDefaults.%'`); err != nil {
		return fmt.Errorf("drop old share links: %w", err)
	}
	return nil
}
