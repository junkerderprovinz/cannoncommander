// Package search searches the index by name, fields and content.
package search

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract. Operations without a handler answer 501.
// The saved searches get their handlers from prefs, next to the other per
// user data.
func Routes(r *v1.Router) {
	r.Contract("search", contract)
}
