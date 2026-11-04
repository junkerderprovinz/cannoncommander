// Package tools holds the tools that compare, sync, find duplicates and clean up.
package tools

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract. Operations without a handler answer 501.
func Routes(r *v1.Router) {
	r.Contract("tools", contract)
	scans := scanTools{deps: r.Deps()}
	r.Handle("POST /tools/duplicates", scans.duplicates)
	r.Handle("GET /tools/diskusage", scans.diskUsage)
}
