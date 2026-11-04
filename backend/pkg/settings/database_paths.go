package settings

import (
	"os"
	"strings"
)

// ResolveDatabasePaths fills an empty SQLite path from CC_DATABASE_PATH or
// the default file name.
func ResolveDatabasePaths() error {
	db := &Config.Server.DatabaseV2
	if strings.TrimSpace(db.Path) == "" {
		if v := os.Getenv("CC_DATABASE_PATH"); v != "" {
			db.Path = v
		} else {
			db.Path = "cannoncommander.sqlite"
		}
	}
	return nil
}
