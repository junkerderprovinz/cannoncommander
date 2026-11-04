package labels

import "github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"

func init() {
	migrate.Register(migrate.Main,
		migrate.SQL("labels/001_create",
			`CREATE TABLE label_colours (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				hex TEXT NOT NULL,
				pos INTEGER NOT NULL
			)`,
			// The six that come with the app have no name of their own, so
			// every user reads them in their language until someone renames one.
			`INSERT INTO label_colours (id, name, hex, pos) VALUES
				('red', '', '#ff6b72', 0),
				('orange', '', '#ff8a3d', 1),
				('yellow', '', '#f5c518', 2),
				('green', '', '#4cc977', 3),
				('blue', '', '#3b9cf5', 4),
				('purple', '', '#a98bff', 5)`,
			// Tags created in the tag list before any file carries them.
			`CREATE TABLE label_tags (
				name TEXT PRIMARY KEY
			)`,
			`CREATE TABLE label_items (
				path TEXT PRIMARY KEY,
				dir TEXT NOT NULL,
				colour TEXT,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE INDEX idx_label_items_dir ON label_items(dir)`,
			`CREATE TABLE label_item_tags (
				path TEXT NOT NULL,
				tag TEXT NOT NULL,
				PRIMARY KEY (path, tag)
			)`,
			`CREATE INDEX idx_label_item_tags_tag ON label_item_tags(tag)`,
		),
	)
}
