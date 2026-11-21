package accounts

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/parleyport/kit/signin/sqlstore"
)

// docsTable keeps the sign-in's documents.
const docsTable = "signin_docs"

func init() {
	migrate.Register(migrate.Main,
		// sessions, recovery_codes, token_use and oidc_identities are how the
		// version before signed people in. They stay for one release so that
		// version can still run on the database.
		migrate.SQL("accounts/001_sessions",
			`CREATE TABLE sessions (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				token_hash TEXT NOT NULL UNIQUE,
				created_at INTEGER NOT NULL,
				seen_at INTEGER NOT NULL,
				user_agent TEXT NOT NULL,
				ip TEXT NOT NULL
			)`,
			`CREATE INDEX idx_sessions_user ON sessions(user_id)`,
		),
		// What the app adds to an account lives here, keyed by the same id.
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
		migrate.SQL("accounts/008_oidc_identities",
			`CREATE TABLE oidc_identities (
				issuer TEXT NOT NULL,
				subject TEXT NOT NULL,
				user_id TEXT NOT NULL UNIQUE,
				PRIMARY KEY (issuer, subject)
			)`,
		),
		// A sign-in service adds people to groups and takes them out again,
		// but only where it put them, so a membership an admin gave stays.
		migrate.SQL("accounts/009_synced_members",
			`CREATE TABLE synced_members (
				group_name TEXT NOT NULL,
				username TEXT NOT NULL,
				PRIMARY KEY (group_name, username)
			)`,
		),
		migrate.SQL("signin/001_docs", sqlstore.Schema(docsTable)),
	)
}
