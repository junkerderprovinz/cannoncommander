package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// EndpointPath is where an assistant talks to the server, below the base
// address of the app.
const EndpointPath = "mcp"

const (
	maxBody        = 1 << 20
	callsPerMinute = 120
	touchEvery     = time.Minute
	authLogEvery   = 10 * time.Second
	// authLogMax is how many addresses the log throttle remembers. Everything
	// older than authLogEvery is dead weight, so a sweep at this mark almost
	// always empties the map; the cap holds a caller rotating through an
	// address range to a fixed amount of memory.
	authLogMax = 1024

	// Wrong keys from one address before it waits a minute, and how many
	// addresses the count remembers.
	authFailsPerMinute = 10
	authFailAddrs      = 4096

	authRealm = `Bearer realm="cannoncommander-mcp"`

	// The JSON-RPC 2.0 code for a request the server refuses to look at.
	invalidRequest = -32600
)

// Login is what the endpoint needs from the sign-in system. The accounts
// provide it; tests put in their own.
type Login interface {
	SessionHash(r *http.Request) (string, bool)
	ByAPIToken(r *http.Request) bool
	ClientIP(r *http.Request) string
	Confirm(r *http.Request, u *users.User, password string) error
}

type accountsLogin struct{ deps v1.Deps }

func (accountsLogin) SessionHash(r *http.Request) (string, bool) { return accounts.SessionHash(r) }
func (accountsLogin) ByAPIToken(r *http.Request) bool            { return accounts.ByAPIToken(r) }
func (accountsLogin) ClientIP(r *http.Request) string            { return accounts.ClientIP(r) }
func (l accountsLogin) Confirm(r *http.Request, u *users.User, password string) error {
	return accounts.ConfirmOwner(r, l.deps.DB, u, password)
}

// Server is the MCP endpoint of one database and what it keeps between
// requests.
type Server struct {
	deps  v1.Deps
	st    store
	login Login

	// user finds a key's owner as the account is now, so a changed role or a
	// new rule counts from the next call on.
	user func(id uint64) (*users.User, error)
	// pepper is the server's signing key, which the digests are made with.
	pepper func() []byte
	// signIn reports whether the server asks people to sign in. Without it
	// nobody could give consent, and a key may only be made from an address
	// no public name can point at.
	signIn func() bool
	now    func() time.Time

	mu        sync.RWMutex
	api       http.Handler
	transport http.Handler
	base      string

	calls      *window
	writes     *window
	authFails  *window
	contentMu  sync.Mutex
	contentRun map[string]string

	touchMu sync.Mutex
	touched map[string]int64
	refused map[string]int64

	authLogMu sync.Mutex
	authLog   map[string]int64

	countMu   sync.Mutex
	requests  map[string]uint64
	toolCalls map[string]uint64

	oauth *oauthState
}

// For returns the server of the database in d, or nil without one.
func For(d v1.Deps) *Server {
	if d.DB == nil {
		return nil
	}
	s, err := v1.Shared(d, "mcp", func() (*Server, error) { return newServer(d), nil })
	if err != nil {
		return nil
	}
	return s
}

func newServer(d v1.Deps) *Server {
	fails := newWindow(time.Minute, authFailsPerMinute)
	fails.maxKeys = authFailAddrs
	s := &Server{
		deps:  d,
		st:    store{db: d.DB},
		login: accountsLogin{deps: d},
		user: func(id uint64) (*users.User, error) {
			u, err := state.GetUserByID(id)
			if err != nil {
				return nil, err
			}
			return &u, nil
		},
		pepper:     func() []byte { return []byte(settings.Config.Auth.Key) },
		signIn:     func() bool { return !state.OpenAccess() },
		now:        time.Now,
		calls:      newWindow(time.Minute, callsPerMinute),
		writes:     newWindow(time.Hour, itemsPerHour),
		authFails:  fails,
		contentRun: map[string]string{},
		touched:    map[string]int64{},
		refused:    map[string]int64{},
		authLog:    map[string]int64{},
		requests:   map[string]uint64{},
		toolCalls:  map[string]uint64{},
		oauth:      newOAuthState(),
	}
	s.transport = s.newTransport()
	return s
}

// Mount puts the endpoint and the public half of the authorization server on
// router, below base. api is the handler of /api/v1 without its prefix, which
// every tool calls with the rights of the key's owner.
func (s *Server) Mount(router *http.ServeMux, base string, api http.Handler) {
	s.mu.Lock()
	s.api, s.base = api, base
	s.mu.Unlock()
	router.HandleFunc(base+EndpointPath, s.serveMCP)
	router.HandleFunc("GET "+resourceMetaPath+base+EndpointPath, s.protectedResource)
	router.HandleFunc("GET "+serverMetaPath, s.serverMetadata)
	router.HandleFunc("POST "+base+registerPath, s.register)
	router.HandleFunc("POST "+base+tokenPath, s.token)
	router.HandleFunc("POST "+base+revokePath, s.revoke)
}

func (s *Server) apiHandler() http.Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.api
}

func (s *Server) basePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.base == "" {
		return "/"
	}
	return s.base
}

// caller is the key a request came in with and the account behind it. The
// gate puts it into the request's context, which is how a tool learns who is
// asking.
type caller struct {
	key  Key
	user *users.User
	addr string
}

type callerKey struct{}

func callerFrom(ctx context.Context) (*caller, bool) {
	c, ok := ctx.Value(callerKey{}).(*caller)
	return c, ok
}

// newTransport builds the protocol server and the transport in front of it.
func (s *Server) newTransport() http.Handler {
	srv := s.newMCPServer()
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		// The SDK answers 400 for nil. Only a request that skipped the gate
		// arrives without a caller.
		if _, ok := callerFrom(r.Context()); !ok {
			return nil
		}
		return srv
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The SDK's rule refuses a request that reached a loopback address
		// under any other name, before any key is read, which is what a reverse
		// proxy in the same network namespace looks like. The key is the
		// defence here: a page that rebinds its name cannot know one, and
		// without a sign-in it cannot make one either (keysAllowedFrom).
		DisableLocalhostProtection:   true,
		MaxRequestBodyBytes:          maxBody,
		PropagateRequestCancellation: true,
	})
}

// serveMCP is the gate in front of the transport: whether the endpoint is on
// at all, method, origin, the address throttle, key or access token, the
// account behind it, the key's budget and the shape of the body, in that
// order.
func (s *Server) serveMCP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	keys, err := s.st.anyActiveKey(ctx)
	if err != nil {
		logger.Errorf("mcp: read the keys: %v", err)
		s.count("unavailable")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "the MCP server is not available right now"})
		return
	}
	issuer, oauthOn := s.issuer(ctx)
	if !keys && !oauthOn {
		s.count("not_found")
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		s.count("method")
		w.Header().Set("Allow", http.MethodPost)
		writeRPCError(w, http.StatusMethodNotAllowed, invalidRequest, "this endpoint takes POST requests with one JSON-RPC message each")
		return
	}
	if foreignOrigin(r) {
		s.count("forbidden_origin")
		writeRPCError(w, http.StatusForbidden, invalidRequest, "cross-origin requests are not accepted")
		return
	}

	addr := s.login.ClientIP(r)
	now := s.now()
	presented, present, conflict := presentedKey(r)
	// An access token is left out of the address throttle both ways. A client
	// presents its expired one as a matter of course, and behind a proxy the
	// failures of anybody else would otherwise lock every connector out, for
	// the sake of a 256-bit secret nobody can guess.
	byToken := oauthOn && strings.HasPrefix(presented, accessPrefix)
	if !byToken && s.authFails.left(addr, now) <= 0 {
		s.count("throttled")
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many wrong keys from this address, wait a minute"})
		return
	}
	challenge := challengeFor(issuer, s.basePath(), oauthOn)
	if !present {
		s.count("no_key")
		w.Header().Set("WWW-Authenticate", challenge)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "an MCP key is required"})
		return
	}
	k, ok := s.match(ctx, presented, byToken, issuer, now)
	var u *users.User
	if ok && !conflict {
		if u, err = s.user(k.UserID); err != nil || u.Username == "" {
			ok = false
		}
	}
	if conflict || !ok {
		if !byToken {
			s.authFails.allow(addr, now)
		}
		s.logAuthFailure(addr, conflict)
		s.count("invalid_key")
		w.Header().Set("WWW-Authenticate", challenge+`, error="invalid_token"`)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "the MCP key is not valid"})
		return
	}

	// A success leaves the failure count where it is. Behind a shared proxy
	// an assistant at work would otherwise wipe a guesser's count with every
	// call, and the throttle would never engage.
	if allowed, retry := s.calls.allow(k.ID, now); !allowed {
		s.count("rate_limited")
		s.recordRefusal(k.ID, "rate_limited", now)
		w.Header().Set("Retry-After", seconds(retry))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many requests for this key, wait a moment"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	// A body that already declares more than the limit is refused for its
	// size by the transport, whatever its first byte says.
	if r.ContentLength <= maxBody {
		if batch, _ := isBatch(r); batch {
			s.count("batch_refused")
			s.recordRefusal(k.ID, "batch_refused", now)
			writeRPCError(w, http.StatusBadRequest, invalidRequest, "batch requests are not accepted")
			return
		}
	}

	s.touch(k, addr, now)
	s.count("ok")
	ctx = context.WithValue(ctx, callerKey{}, &caller{key: k, user: u, addr: addr})
	s.transport.ServeHTTP(w, r.WithContext(ctx))
}

// match finds the key or the grant behind what the request presented.
func (s *Server) match(ctx context.Context, presented string, byToken bool, issuer string, now time.Time) (Key, bool) {
	pepper := s.pepper()
	if len(pepper) == 0 {
		return Key{}, false
	}
	if byToken {
		g, err := s.st.accessGrant(ctx, hashOAuth(pepper, "access", presented), now.Unix())
		if err != nil {
			if !errors.Is(err, errTokenNotFound) {
				logger.Errorf("mcp: look up an access token: %v", err)
			}
			return Key{}, false
		}
		return g, g.Resource == resourceOf(issuer, s.basePath())
	}
	if !strings.HasPrefix(presented, keyPrefix) {
		return Key{}, false
	}
	digest := hashKey(pepper, presented)
	k, err := s.st.keyByDigest(ctx, digest)
	if err != nil {
		if !errors.Is(err, errKeyNotFound) {
			logger.Errorf("mcp: look up a key: %v", err)
		}
		return Key{}, false
	}
	return k, sameString(digest, k.Digest)
}

// presentedKey reads the key from Authorization: Bearer or from X-API-Key,
// which mcp-remote is told to send because its argument handling breaks a
// header value with a space on some systems. conflict means both arrived and
// disagree.
func presentedKey(r *http.Request) (key string, present, conflict bool) {
	var bearer string
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		if scheme, rest, found := strings.Cut(v, " "); found && strings.EqualFold(scheme, "bearer") {
			bearer = strings.TrimSpace(rest)
		}
	}
	apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
	switch {
	case bearer != "" && apiKey != "":
		return bearer, true, bearer != apiKey
	case bearer != "":
		return bearer, true, false
	case apiKey != "":
		return apiKey, true, false
	}
	return "", false, false
}

// foreignOrigin reports an Origin header that is present and names something
// other than the host the request reached. It does not catch DNS rebinding,
// whose page carries the attacker's name in both; the key does that.
func foreignOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if origin == "null" || err != nil || u.Host == "" {
		return true
	}
	return !strings.EqualFold(u.Host, r.Host)
}

// touch records when and from where a key was last used, at most once per
// touchEvery, so an assistant at work does not add a write per request.
func (s *Server) touch(k Key, addr string, now time.Time) {
	s.touchMu.Lock()
	recent := now.Unix()-s.touched[k.ID] < int64(touchEvery.Seconds())
	if !recent {
		s.touched[k.ID] = now.Unix()
	}
	s.touchMu.Unlock()
	if recent {
		return
	}
	if err := s.st.touchKey(context.Background(), k.ID, now.Unix(), addr); err != nil {
		logger.Errorf("mcp: key %s: record its last use: %v", k.ID, err)
	}
}

// recordRefusal puts a request the gate turned away into the key's log, at
// most once per touchEvery for each key and reason, so a client looping on a
// refusal does not keep the database busy writing about it.
func (s *Server) recordRefusal(keyID, outcome string, now time.Time) {
	mark := keyID + "|" + outcome
	s.touchMu.Lock()
	recent := now.Unix()-s.refused[mark] < int64(touchEvery.Seconds())
	if !recent {
		s.refused[mark] = now.Unix()
	}
	s.touchMu.Unlock()
	if !recent {
		s.recordEvent(keyID, Event{At: now.Unix(), Outcome: outcome, routine: true})
	}
}

// recordEvent writes one entry of a key's log. Like the last use it must not
// fail the request.
func (s *Server) recordEvent(keyID string, e Event) {
	if err := s.st.recordEvent(context.Background(), keyID, e); err != nil {
		logger.Errorf("mcp: key %s: record %q -> %s: %v", keyID, e.Tool, e.Outcome, err)
	}
}

// logAuthFailure names the address and the reason at most once per
// authLogEvery per address, so a looping client cannot crowd everything else
// out of the log.
func (s *Server) logAuthFailure(addr string, conflict bool) {
	now := s.now().Unix()
	cutoff := now - int64(authLogEvery.Seconds())
	s.authLogMu.Lock()
	due := s.authLog[addr] <= cutoff
	if due && len(s.authLog) >= authLogMax {
		for seen, at := range s.authLog {
			if at <= cutoff {
				delete(s.authLog, seen)
			}
		}
		due = len(s.authLog) < authLogMax
	}
	if due {
		s.authLog[addr] = now
	}
	s.authLogMu.Unlock()
	if !due {
		return
	}
	reason := "the key does not match any active one"
	if conflict {
		reason = "Authorization and X-API-Key carry different keys"
	}
	logger.Warningf("mcp: refused a request from %s: %s", addr, reason)
}

func (s *Server) count(outcome string) {
	s.countMu.Lock()
	s.requests[outcome]++
	s.countMu.Unlock()
}

func (s *Server) countTool(tool, outcome string) {
	s.countMu.Lock()
	s.toolCalls[tool+"|"+outcome]++
	s.countMu.Unlock()
}

// counters returns how the gate answered and how the tools went since the
// server started.
func (s *Server) counters() (requests, tools map[string]uint64) {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	requests, tools = make(map[string]uint64, len(s.requests)), make(map[string]uint64, len(s.toolCalls))
	for k, v := range s.requests {
		requests[k] = v
	}
	for k, v := range s.toolCalls {
		tools[k] = v
	}
	return requests, tools
}

// writeRPCError answers with a JSON-RPC error envelope. The id is null because
// the gate decides before the request is parsed.
func writeRPCError(w http.ResponseWriter, status, code int, msg string) {
	writeJSON(w, status, map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": code, "message": msg}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// rewound hands the bytes isBatch read back to the next reader ahead of the
// rest of the body.
type rewound struct {
	io.Reader
	io.Closer
}

// isBatch reports whether the body's first byte past whitespace is '[', the
// shape of a JSON-RPC batch. What it read goes back in front of the rest, so
// the transport reads the body unchanged, and a read error travels with it.
func isBatch(r *http.Request) (bool, error) {
	var peeked []byte
	rest := r.Body
	defer func() { r.Body = rewound{Reader: io.MultiReader(bytes.NewReader(peeked), rest), Closer: rest} }()
	buf := make([]byte, 512)
	for {
		n, err := rest.Read(buf)
		peeked = append(peeked, buf[:n]...)
		if i := bytes.IndexFunc(peeked, func(c rune) bool { return c != ' ' && c != '\t' && c != '\n' && c != '\r' }); i >= 0 {
			return peeked[i] == '[', nil
		}
		if err != nil {
			return false, err
		}
	}
}

func seconds(d time.Duration) string {
	n := int((d + time.Second - 1) / time.Second)
	if n < 1 {
		n = 1
	}
	return strconv.Itoa(n)
}
