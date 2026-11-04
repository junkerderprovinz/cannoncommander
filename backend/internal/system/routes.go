// Package system reports the version and state of the server and its settings.
package system

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract. Operations without a handler answer 501.
func Routes(r *v1.Router) {
	r.Contract("system", contract)
	search.IndexRoutes(r)
}
