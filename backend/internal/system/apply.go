package system

import (
	"context"
	"database/sql"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// ApplyStored puts the saved settings into the running config. It runs
// before the web server starts, so the first request already sees them.
func ApplyStored(ctx context.Context, db *sql.DB) {
	if db != nil {
		a, saved, err := readApp(ctx, db)
		if err != nil {
			logger.Errorf("app settings: %v", err)
		} else if saved {
			apply(a)
		}
	}
}

// apply puts settings into effect in the running server.
func apply(a App) {
	c := &settings.Config
	oo := &c.Integrations.OnlyOffice
	switch a.Office.Kind {
	case OfficeOnlyOffice, OfficeEuroOffice:
		oo.Url, oo.Secret = a.Office.URL, a.Office.Secret
	default:
		oo.Url, oo.Secret = "", ""
	}
	c.Server.DisablePreviews = !a.Previews.Thumbs
	c.Http.DisableWebDAV = !a.WebDAV
	if err := settings.SetLogLevel(a.LogLevel); err != nil {
		logger.Errorf("log level: %v", err)
	}
}
