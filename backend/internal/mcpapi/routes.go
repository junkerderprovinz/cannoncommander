// Package mcpapi is the MCP server: an endpoint an assistant such as Claude
// or ChatGPT talks to, the keys it signs in with and an OAuth authorization
// server for the clients that can only sign in that way. Every key works for
// one account and reaches the files through /api/v1 as that account, so it
// never sees or does more than its owner could in the web interface. A key
// reads; only a key with its write switch on changes files, and only through
// the jobs the web interface runs, so every change can be followed and undone.
package mcpapi

import (
	"context"
	_ "embed"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// A deleted account takes its keys along, before anything else could reuse
// what hangs on it.
func init() {
	accounts.OnDelete(func(ctx context.Context, d v1.Deps, userID uint64) error {
		if d.DB == nil {
			return nil
		}
		return store{db: d.DB}.forgetUser(ctx, userID)
	})
}

// Routes registers the contract and, when there is a database, the handlers
// that manage the keys. The endpoint itself lives outside /api/v1; Mount puts
// it on the server.
func Routes(r *v1.Router) { routesOf(r, For(r.Deps())) }

func routesOf(r *v1.Router, s *Server) {
	r.Contract("mcp", contract)
	if s == nil {
		return
	}
	r.Handle("GET /mcp/keys", s.list)
	r.Handle("POST /mcp/keys", s.create)
	r.Handle("PATCH /mcp/keys/{id}", s.update)
	r.Handle("DELETE /mcp/keys/{id}", s.purge)
	r.Handle("POST /mcp/keys/{id}/rotate", s.rotate)
	r.Handle("POST /mcp/keys/{id}/revoke", s.revokeOne)
	r.Handle("GET /mcp/keys/{id}/activity", s.activity)
	r.Handle("PUT /mcp/oauth", s.setOAuth)
	r.Handle("GET /mcp/oauth/authorize", s.consentInfo)
	r.Handle("POST /mcp/oauth/authorize", s.consent)
}
