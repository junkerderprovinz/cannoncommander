package prefs

import "github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"

func init() {
	migrate.Register(migrate.Main,
		migrate.SQL("prefs/001_create",
			`CREATE TABLE user_prefs (
				user_id TEXT PRIMARY KEY,
				data TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE TABLE device_prefs (
				user_id TEXT NOT NULL,
				device TEXT NOT NULL,
				data TEXT NOT NULL,
				updated_at INTEGER NOT NULL,
				PRIMARY KEY (user_id, device)
			)`,
			// Bookmarks and rename presets are always saved as a whole list,
			// so one document per user keeps their order without bookkeeping.
			`CREATE TABLE bookmarks (
				user_id TEXT PRIMARY KEY,
				items TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE TABLE rename_presets (
				user_id TEXT PRIMARY KEY,
				items TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE TABLE profiles (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id TEXT NOT NULL,
				name TEXT NOT NULL,
				layout TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE INDEX idx_profiles_user ON profiles(user_id)`,
			`CREATE TABLE saved_searches (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id TEXT NOT NULL,
				name TEXT NOT NULL,
				query TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE INDEX idx_saved_searches_user ON saved_searches(user_id)`,
			`CREATE TABLE frequent_dirs (
				user_id TEXT NOT NULL,
				path TEXT NOT NULL,
				visits INTEGER NOT NULL,
				last_at INTEGER NOT NULL,
				PRIMARY KEY (user_id, path)
			)`,
		),
	)
}
