// Command installer installs CannonCommander on Windows and Linux. The desktop
// build (scripts/desktop.mjs) copies the program into payload/ and builds this
// around it; on macOS the app ships as a zipped bundle instead.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/junkerderprovinz/pommelporter"
)

var (
	//go:embed payload
	payload embed.FS
	//go:embed logo.svg
	logo []byte
	//go:embed icon.png
	icon []byte
)

// Set by the build: an updatetest build installs as "CannonCommander Test",
// next to a real installation.
var (
	name    = "CannonCommander"
	id      = "com.junkerderprovinz.cannoncommander"
	version = "0.0.0"
)

func main() {
	files, err := fs.Sub(payload, "payload")
	if err == nil {
		err = pommelporter.Run(pommelporter.App{
			Name:       name,
			ID:         id,
			Executable: executable,
			Version:    version,
			Publisher:  "junkerderprovinz",
			Homepage:   "https://github.com/junkerderprovinz/cannoncommander",
			Accent:     "#FCC419",
			Logo:       logo,
			Icon:       icon,
			Categories: []string{"System", "FileManager"},
			Payload:    files,
			// The lock a copy on its usual data folder holds, instanceID in
			// desktop/main.go.
			InstanceID: id,
			Choices:    []pommelporter.Choice{fileManager},
			Stays:      stays,
			Steps:      []pommelporter.Step{helperPrograms},
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
