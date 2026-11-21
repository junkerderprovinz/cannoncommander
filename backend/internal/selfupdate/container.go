//go:build !desktop

package selfupdate

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes serves the contract without the feature: a container is updated by
// its container manager.
func Routes(r *v1.Router) { r.DesktopContract("selfupdate", contract) }
