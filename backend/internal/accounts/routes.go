// Package accounts manages the own account and, for admins, users, groups and rules.
package accounts

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers that exist. Operations
// without a handler answer 501.
func Routes(r *v1.Router) {
	r.Contract("accounts", contract)
	s := sessionHandlers{db: r.Deps().DB}
	r.Handle("GET /me/sessions", s.list)
	r.Handle("DELETE /me/sessions/{id}", s.end)
}
