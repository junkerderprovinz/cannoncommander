package v1

import (
	"context"
	"net/http"
)

// Agent is an assistant that works for the signed-in user through a key of
// the MCP server. Jobs carry its name, so the user can tell its work from
// their own.
type Agent struct {
	KeyID string
	Name  string
}

type agentKey struct{}

// WithAgent marks what runs under ctx as the work of a.
func WithAgent(ctx context.Context, a Agent) context.Context {
	return context.WithValue(ctx, agentKey{}, a)
}

// AgentFrom returns the assistant behind r, if one sent it.
func AgentFrom(r *http.Request) (Agent, bool) {
	a, ok := r.Context().Value(agentKey{}).(Agent)
	return a, ok
}
