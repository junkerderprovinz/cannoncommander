package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gtsteffaniak/go-logger/logger"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// CannonCommander is the authorization server of its own MCP endpoint, so a
// client that can only sign in through OAuth reaches it too. It follows the
// MCP authorization specification: protected resource metadata (RFC 9728),
// authorization server metadata (RFC 8414), dynamic client registration
// (RFC 7591), the authorization code grant with PKCE S256, resource
// indicators (RFC 8707), the iss response parameter (RFC 9207) and token
// revocation (RFC 7009). The user who signs in on the consent page is the
// account the client then works for.
const (
	scope = "mcp"

	authorizePath    = "oauth/authorize"
	tokenPath        = "oauth/token" //nolint:gosec // a path, not a credential
	registerPath     = "oauth/register"
	revokePath       = "oauth/revoke"
	serverMetaPath   = "/.well-known/oauth-authorization-server"
	resourceMetaPath = "/.well-known/oauth-protected-resource"

	accessTTL  = time.Hour
	refreshTTL = 30 * 24 * time.Hour
	codeTTL    = 2 * time.Minute
	consentTTL = 10 * time.Minute

	// A code waits here between the consent and the client's exchange, which
	// takes seconds. Only a signed-in user can add one, so the cap is about a
	// stuck client, not an attacker.
	pendingMax = 64

	// Registration needs no credentials. Per address, or per /64 for IPv6,
	// it is held to a few an hour; the store caps the clients nobody signed
	// in with.
	registrationsPerHour = 10
	registrationAddrs    = 4096

	formMax = 16 << 10
)

var (
	errOAuthNeedsSignIn = errors.New("sign-in through OAuth needs a server people sign in to")
	errOAuthIssuer      = errors.New("the public address must be an https address without a path, such as https://files.example.com")
)

// oauthState is what the authorization server keeps between requests. Codes
// and spent consent tickets live only here: a restart ends a sign-in half
// way, which the client simply starts again.
type oauthState struct {
	mu            sync.Mutex
	codes         map[string]*pendingCode
	tickets       map[string]int64
	registrations *window
}

// pendingCode is a code handed to a client after a user allowed it, keyed by
// its digest. used stays set until it expires, so a code presented twice is
// recognised and the grant it made revoked.
type pendingCode struct {
	clientID    string
	redirectURI string
	challenge   string
	resource    string
	label       string
	known       string
	userID      uint64
	canWrite    bool
	expires     time.Time
	used        bool
	grantID     string
}

func newOAuthState() *oauthState {
	reg := newWindow(time.Hour, registrationsPerHour)
	reg.maxKeys = registrationAddrs
	return &oauthState{codes: map[string]*pendingCode{}, tickets: map[string]int64{}, registrations: reg}
}

// registrationKey is the bucket a registration from addr counts in. An IPv6
// host usually has a whole /64, so its addresses share one bucket.
func registrationKey(addr string) string {
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}

// oauthClient is a client that registered itself. SecretDigest is the HMAC
// of its secret, empty for a public client. Known is the id of a client the
// card has a mark for, decided by the redirect URIs and never by the name the
// client sent.
type oauthClient struct {
	ID           string
	Name         string
	RedirectURIs []string
	AuthMethod   string
	SecretDigest string
	Known        string
	CreatedAt    int64
	CreatedFrom  string
}

func (c oauthClient) redirectJSON() string {
	raw, _ := json.Marshal(c.RedirectURIs)
	return string(raw)
}

func (c *oauthClient) readRedirects(raw string) error { return json.Unmarshal([]byte(raw), &c.RedirectURIs) }

// knownClients are the clients the card and the consent page show with their
// own mark. A client counts as one of them only when every redirect URI it
// registered is the vendor's own callback: a name is whatever the caller of
// the registration typed, and a vendor's domain also serves pages its users
// write.
var knownClients = []struct {
	id        string
	name      string
	callbacks []string
}{
	{"chatgpt", "ChatGPT", []string{"https://chatgpt.com/connector_platform_oauth_redirect"}},
	{"claude", "Claude", []string{"https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback"}},
}

func knownClient(uris []string) string {
	found := ""
	for _, uri := range uris {
		id := ""
		for _, k := range knownClients {
			if slices.Contains(k.callbacks, uri) {
				id = k.id
			}
		}
		if id == "" || (found != "" && found != id) {
			return ""
		}
		found = id
	}
	return found
}

func knownClientName(id string) string {
	for _, k := range knownClients {
		if k.id == id {
			return k.name
		}
	}
	return ""
}

// oauthPossible reports whether this server could offer OAuth at all: only a
// server people sign in to can ask one of them for consent, and the desktop
// app has nobody to sign in.
func (s *Server) oauthPossible() bool { return s.signIn() && !desktop.Active() }

// issuer returns the issuer while OAuth is offered: possible, switched on and
// with a public address stored.
func (s *Server) issuer(ctx context.Context) (string, bool) {
	if !s.oauthPossible() {
		return "", false
	}
	o, err := s.st.oauthSettings(ctx)
	if err != nil {
		logger.Errorf("mcp: read the OAuth settings: %v", err)
		return "", false
	}
	return o.Issuer, o.Enabled && o.Issuer != ""
}

// resourceOf is the canonical address of the endpoint under issuer, the
// audience of every token.
func resourceOf(issuer, base string) string { return issuer + base + EndpointPath }

// normalizeIssuer turns what an admin typed into the one spelling clients
// compare the issuer in: https, a lower-case host, no default port and nothing
// after the host.
func normalizeIssuer(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsAny(raw, "?#") || !strings.EqualFold(u.Scheme, "https") ||
		u.Opaque != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if host == "" {
		return "", false
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" && port != "443" {
		host += ":" + port
	}
	return "https://" + host, true
}

// sameResource reports whether a resource parameter names want, allowing for
// the spellings RFC 8707 leaves to clients: the case of scheme and host, a
// default port and a trailing slash.
func sameResource(given, want string) bool {
	u, err := url.Parse(given)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	base, ok := normalizeIssuer(u.Scheme + "://" + u.Host)
	return ok && base+strings.TrimSuffix(u.Path, "/") == strings.TrimSuffix(want, "/")
}

// validRedirect reports whether a client may register uri: https with a
// host, or plain http to a loopback address for a client on the user's own
// computer, never with a fragment or credentials. The host has to be plain
// ASCII, so the consent page never shows a lookalike of a familiar name.
func validRedirect(uri string) bool {
	if len(uri) > 512 || strings.Contains(uri, "#") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Hostname() == "" ||
		strings.IndexFunc(u.Host, func(r rune) bool { return r > unicode.MaxASCII || r == '%' }) >= 0 {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		return loopback(u.Hostname())
	}
	return false
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// redirectRegistered reports whether given is one of the registered redirect
// URIs. The match is exact, except that a loopback redirect may name any
// port: a native client listens on whatever port is free (RFC 8252 7.3).
func redirectRegistered(registered []string, given string) bool {
	for _, r := range registered {
		if r == given || loopbackMatch(r, given) {
			return true
		}
	}
	return false
}

func loopbackMatch(registered, given string) bool {
	a, errA := url.Parse(registered)
	b, errB := url.Parse(given)
	if errA != nil || errB != nil || a.Scheme != "http" || b.Scheme != "http" || !loopback(a.Hostname()) {
		return false
	}
	return a.Hostname() == b.Hostname() && a.Path == b.Path && a.RawQuery == b.RawQuery && b.User == nil && !strings.Contains(given, "#")
}

// cleanClientName keeps what a client calls itself to 64 printable
// characters, because the consent page and the card show it. Format
// characters go too: they include the ones that reorder text or join letters
// invisibly.
func cleanClientName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if utf8.RuneCountInString(s) > 64 {
		s = string([]rune(s)[:64])
	}
	return strings.TrimSpace(s)
}

// challengeFor is the WWW-Authenticate value of a 401 from the endpoint.
// While OAuth is offered it names the protected resource metadata, which is
// how a client learns that it can sign in.
func challengeFor(issuer, base string, on bool) string {
	if !on {
		return authRealm
	}
	return authRealm + `, resource_metadata="` + issuer + resourceMetaPath + base + EndpointPath + `", scope="` + scope + `"`
}

// protectedResource serves the RFC 9728 metadata of the endpoint.
func (s *Server) protectedResource(w http.ResponseWriter, r *http.Request) {
	issuer, on := s.issuer(r.Context())
	if !on {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 resourceOf(issuer, s.basePath()),
		"authorization_servers":    []string{issuer},
		"scopes_supported":         []string{scope},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "CannonCommander",
	})
}

// authMethods are the ways a client authenticates at the token and revocation
// endpoints. Cloud connectors register as public clients and rely on PKCE; a
// client that asks for a secret gets one.
var authMethods = []string{"none", "client_secret_basic", "client_secret_post"}

// serverMetadata serves the RFC 8414 metadata of the authorization server.
func (s *Server) serverMetadata(w http.ResponseWriter, r *http.Request) {
	issuer, on := s.issuer(r.Context())
	if !on {
		http.NotFound(w, r)
		return
	}
	base := issuer + s.basePath()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         issuer,
		"authorization_endpoint":                         base + authorizePath,
		"token_endpoint":                                 base + tokenPath,
		"registration_endpoint":                          base + registerPath,
		"revocation_endpoint":                            base + revokePath,
		"scopes_supported":                               []string{scope},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          authMethods,
		"revocation_endpoint_auth_methods_supported":     authMethods,
		"authorization_response_iss_parameter_supported": true,
	})
}
