package web

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
)

// deriveRPID returns the relying party id for the host the browser asked
// for, or "" when that host cannot carry a passkey: browsers refuse them on
// a bare address. localhost works, as browsers treat it as a secure place.
func deriveRPID(r *http.Request) string {
	host := requestHost(r)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if net.ParseIP(host) != nil {
		return ""
	}
	return host
}

func webAuthnRequestOrigin(r *http.Request) string {
	return fmt.Sprintf("%s://%s", requestScheme(r), requestHost(r))
}

// registerRequestOrigin registers the request's origin and RP ID with the
// WebAuthn service and returns the RP ID, "" for a host without passkeys.
func registerRequestOrigin(r *http.Request) string {
	rpID := deriveRPID(r)
	if svc := auth.GetWebAuthn(); svc != nil && rpID != "" {
		svc.EnsureOrigin(webAuthnRequestOrigin(r))
		svc.EnsureRPID(rpID)
	}
	return rpID
}
