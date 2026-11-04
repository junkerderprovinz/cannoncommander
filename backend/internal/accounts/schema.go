package accounts

import "github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"

func init() {
	migrate.Register(migrate.Main,
		// The login system keeps users in its own store. What the app adds to
		// an account lives here, keyed by the same id.
		migrate.SQL("accounts/002_profiles",
			`CREATE TABLE account_profiles (
				user_id TEXT PRIMARY KEY,
				name TEXT NOT NULL DEFAULT '',
				password_at INTEGER NOT NULL DEFAULT 0,
				seen_at INTEGER NOT NULL DEFAULT 0
			)`,
		),
		migrate.SQL("accounts/003_recovery_codes",
			`CREATE TABLE recovery_codes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id TEXT NOT NULL,
				code_hash TEXT NOT NULL,
				used_at INTEGER NOT NULL DEFAULT 0
			)`,
			`CREATE INDEX idx_recovery_codes_user ON recovery_codes(user_id)`,
		),
		migrate.SQL("accounts/004_token_use",
			`CREATE TABLE token_use (
				token_hash TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				seen_at INTEGER NOT NULL
			)`,
			`CREATE INDEX idx_token_use_user ON token_use(user_id)`,
		),
		migrate.SQL("accounts/005_path_rules",
			`CREATE TABLE path_rules (
				pos INTEGER PRIMARY KEY,
				who TEXT NOT NULL,
				path TEXT NOT NULL,
				rule TEXT NOT NULL
			)`,
		),
		migrate.SQL("accounts/006_group_notes",
			`CREATE TABLE group_notes (
				name TEXT PRIMARY KEY,
				note TEXT NOT NULL
			)`,
		),
	)
}
