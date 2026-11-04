// Package jobs runs copy, move and the other long tasks in the background.
package jobs

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract. Operations without a handler answer 501.
func Routes(r *v1.Router) {
	r.Contract("jobs", contract)
}
