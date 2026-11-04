package cmd

import (
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// healthcheck attempt to save test file against configured url
func validateOfficeIntegration() {
	if settings.Config.Integrations.OnlyOffice.Url != "" {
		// get url health
		// get request against the url
		_, err := http.NewRequest("GET", settings.Config.Integrations.OnlyOffice.Url, nil)
		if err != nil {
			logger.Warning("Could not create request to only office Url, make sure its valid and running")
			return
		}
	}
}
