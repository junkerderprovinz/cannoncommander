//go:build !desktop

package modules

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes serves the contract without the feature, which only the desktop app
// has. The image carries its helper programs.
func Routes(r *v1.Router) { r.DesktopContract("modules", contract) }
