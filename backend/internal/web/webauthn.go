package web

import (
	"fmt"
	"net/http"

	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
)

func deriveRPID(r *http.Request) string {
	host := requestHost(r)
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	return host
}

func webAuthnRequestOrigin(r *http.Request) string {
	return fmt.Sprintf("%s://%s", requestScheme(r), requestHost(r))
}

// registerRequestOrigin registers the request's origin and RP ID with the WebAuthn service.
func registerRequestOrigin(r *http.Request) {
	origin := webAuthnRequestOrigin(r)
	rpID := deriveRPID(r)
	svc := auth.GetWebAuthn()
	if svc != nil {
		svc.EnsureOrigin(origin)
		svc.EnsureRPID(rpID)
	}
}
