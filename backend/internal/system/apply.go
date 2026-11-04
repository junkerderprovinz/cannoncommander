package system

import (
	"context"
	"database/sql"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// boot remembers the settings that only take effect at start, so the
// settings page can say when a restart is due.
var boot struct {
	once      sync.Once
	autoCheck bool
}

func captureBoot() {
	boot.once.Do(func() {
		boot.autoCheck = !settings.Config.Server.DisableUpdateCheck
	})
}

func restartNeeded(a App) bool {
	captureBoot()
	return a.Updates.AutoCheck != boot.autoCheck
}

// ApplyStored puts the saved settings into the running config. It runs
// before the web server registers its routes, because the update check is only
// looked at once.
func ApplyStored(ctx context.Context, db *sql.DB) {
	if db != nil {
		a, saved, err := readApp(ctx, db)
		if err != nil {
			logger.Errorf("app settings: %v", err)
		} else if saved {
			apply(a)
		}
	}
	captureBoot()
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
	c.Server.DisableUpdateCheck = !a.Updates.AutoCheck
	if err := settings.SetLogLevel(a.LogLevel); err != nil {
		logger.Errorf("log level: %v", err)
	}
}
