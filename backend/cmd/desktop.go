//go:build desktop

package cmd

import (
	"context"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
)

// Desktop runs the server inside the desktop app until ctx ends. The app
// writes configFile and calls desktop.Use before it calls this.
func Desktop(ctx context.Context, configFile string) {
	configPath = configFile
	serve(ctx, initializeDatabase(configFile, false), nil)
}

// Version is the version the build stamped in.
func Version() string { return version.Version }
