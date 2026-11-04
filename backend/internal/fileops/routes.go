// Package fileops creates and renames files, copies, moves and deletes
// them as jobs, and checks a transfer before it starts.
package fileops

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and, when there is a database, the
// handlers.
func Routes(r *v1.Router) {
	r.Contract("fileops", contract)
	s := For(r.Deps())
	if s == nil {
		return
	}
	r.Handle("POST /fs/mkdir", s.mkdir)
	r.Handle("POST /fs/touch", s.touch)
	r.Handle("POST /fs/rename", s.rename)
	r.Handle("POST /ops/preflight", s.preflight)
}
