package settings

import (
	"os"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// MigrateFromDefault resolves server.database.migrateFrom from CC_DATABASE or database.db.
const MigrateFromDefault = "default"

const defaultLegacyDatabasePath = "database.db"

// ResolveDatabasePaths applies migrateFrom "default" and fills an empty SQLite path from existing defaults.
func ResolveDatabasePaths() error {
	db := &Config.Server.DatabaseV2
	if strings.TrimSpace(db.Path) == "" {
		if v := os.Getenv("CC_DATABASE_PATH"); v != "" {
			db.Path = v
		} else {
			db.Path = "cannoncommander.sqlite"
		}
	}

	migrateFrom := strings.TrimSpace(db.MigrateFrom)
	legacyEnv := strings.TrimSpace(os.Getenv("CC_DATABASE"))

	if migrateFrom == MigrateFromDefault {
		if legacyEnv != "" {
			db.MigrateFrom = legacyEnv
		} else {
			db.MigrateFrom = defaultLegacyDatabasePath
		}
	} else if legacyEnv != "" {
		logger.Fatalf(
			"CC_DATABASE environment variable is deprecated. " +
				`Remove it, or set server.database.migrateFrom to "default" to migrate from that path on first start.`,
		)
	}
	return nil
}
