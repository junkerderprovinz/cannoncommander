package accounts

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

func init() {
	migrate.Register(migrate.Main,
		migrate.SQL("accounts/001_sessions",
			`CREATE TABLE sessions (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				token_hash TEXT NOT NULL UNIQUE,
				created_at INTEGER NOT NULL,
				seen_at INTEGER NOT NULL,
				user_agent TEXT NOT NULL,
				ip TEXT NOT NULL
			)`,
			`CREATE INDEX idx_sessions_user ON sessions(user_id)`,
		),
	)
}

// Login is what the accounts need from the login system.
type Login interface {
	// Session returns the hash of the session token r carries and the
	// client's address. ok is false for API tokens.
	Session(r *http.Request) (hash, ip string, ok bool)
	// Alive reports whether the session token with this hash still logs in.
	Alive(hash string) bool
	// End revokes the session token with this hash.
	End(hash string) error
	// EndAll revokes every session of the user except the one with the
	// token hash keep and returns how many it ended.
	EndAll(userID uint64, keep string) (int, error)
	// SignedInAt returns when the session r carries signed in. ok is false
	// for API tokens.
	SignedInAt(r *http.Request) (at time.Time, ok bool)
	// APIToken returns the API token r logs in with. ok is false for
	// sessions and every other kind of login.
	APIToken(r *http.Request) (raw string, ok bool)
	// Start logs u in on this client with a new session. With keep above
	// zero the session lasts that long, through browser restarts. Without,
	// it ends with the browser.
	Start(w http.ResponseWriter, r *http.Request, u *users.User, keep time.Duration) error
	// Renew gives the session r carries a new token when it stays signed in
	// and its token is due, and returns the hash of the new one. ok is false
	// when the token stays.
	Renew(w http.ResponseWriter, r *http.Request, u *users.User) (hash string, ok bool)
	// Stop ends the session r carries and clears its cookie.
	Stop(w http.ResponseWriter, r *http.Request) error
	// ClientIP returns the address of the client behind any proxy.
	ClientIP(r *http.Request) string
	// PasskeySite readies passkeys for the address r came in on and returns
	// the relying party id and the host as the browser shows it. rpID is ""
	// for an address a browser keeps no passkey for.
	PasskeySite(r *http.Request) (rpID, host string)
}

var login Login

// UseLogin connects the accounts to the login system. The server calls it
// once before it starts serving.
func UseLogin(l Login) { login = l }

// seenEvery limits how often a session's last use is written, since every
// request passes through Track.
const seenEvery = time.Minute

var (
	seenMu sync.Mutex
	seen   = map[string]time.Time{}
)

// Track wraps the authenticator of /api/v1. Every session that makes a
// request shows up in the list with its browser, address and last use, API
// tokens remember when they were used, a read only token gets no further
// than reading, and a session that stays signed in is renewed.
func Track(db *sql.DB, next v1.Authenticator) v1.Authenticator {
	return func(w http.ResponseWriter, r *http.Request) (*users.User, error) {
		u, err := next(w, r)
		if err != nil {
			return u, err
		}
		if raw, ok := login.APIToken(r); ok {
			name, _ := users.TokenNameByRaw(u.Tokens, raw)
			if u.Tokens[name].ReadOnly && !safe(r.Method) {
				return nil, v1.Coded(http.StatusForbidden, "tokenReadOnly", "this API token may only read")
			}
			TokenUsed(r.Context(), db, u, raw)
			return u, nil
		}
		hash, ip, ok := login.Session(r)
		if !ok {
			return u, nil
		}
		if next, renewed := login.Renew(w, r, u); renewed {
			// The session keeps its place in the list under its new token.
			if _, err := db.ExecContext(r.Context(), `UPDATE sessions SET token_hash = ? WHERE token_hash = ?`, next, hash); err != nil {
				logger.Errorf("move renewed session: %v", err)
			}
			hash = next
		}
		if !due(hash) {
			return u, nil
		}
		now := time.Now().Unix()
		ua := r.UserAgent()
		if len(ua) > 512 {
			ua = ua[:512]
		}
		// The list is bookkeeping, so a failed write must not fail the request.
		if _, err := db.ExecContext(r.Context(), `INSERT INTO sessions (id, user_id, token_hash, created_at, seen_at, user_agent, ip)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (token_hash) DO UPDATE SET seen_at = excluded.seen_at, user_agent = excluded.user_agent, ip = excluded.ip`,
			rand.Text(), idOf(u), hash, now, now, ua, ip); err != nil {
			logger.Errorf("record session: %v", err)
		}
		if err := seenNow(r.Context(), db, idOf(u), now); err != nil {
			logger.Errorf("record last login: %v", err)
		}
		return u, nil
	}
}

// TokenUsed notes that the API token raw of u was used just now, for the list
// of tokens. Like the session list, this is bookkeeping that must not fail a
// request.
func TokenUsed(ctx context.Context, db *sql.DB, u *users.User, raw string) {
	hash := utils.HashSHA256(raw)
	if !due("token:" + hash) {
		return
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO token_use (token_hash, user_id, seen_at) VALUES (?, ?, ?)
		ON CONFLICT (token_hash) DO UPDATE SET seen_at = excluded.seen_at`, hash, idOf(u), time.Now().Unix()); err != nil {
		logger.Errorf("record token use: %v", err)
	}
}

func safe(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// SessionHash returns the hash of the session r signed in with. ok is false
// for an API token and for every request without a session.
func SessionHash(r *http.Request) (hash string, ok bool) {
	if login == nil {
		return "", false
	}
	hash, _, ok = login.Session(r)
	return hash, ok
}

// ByAPIToken reports whether r signed in with an API token.
func ByAPIToken(r *http.Request) bool {
	if login == nil {
		return false
	}
	_, ok := login.APIToken(r)
	return ok
}

// ClientIP returns the address of the client behind r, behind any proxy the
// config trusts.
func ClientIP(r *http.Request) string {
	if login == nil {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
	return login.ClientIP(r)
}

// needSession refuses changes to logins and credentials that come with an
// API token, so a token can never mint a stronger one or lock its owner out.
func needSession(r *http.Request) error {
	if _, ok := login.APIToken(r); ok {
		return v1.Coded(http.StatusForbidden, "tokenCannotChange", "sign in to change this, an API token cannot")
	}
	return nil
}

func due(hash string) bool {
	seenMu.Lock()
	defer seenMu.Unlock()
	now := time.Now()
	if last, ok := seen[hash]; ok && now.Sub(last) < seenEvery {
		return false
	}
	// Rotated tokens leave their hashes behind, so the map starts over from
	// time to time instead of growing for as long as the server runs.
	if len(seen) > 10000 {
		clear(seen)
	}
	seen[hash] = now
	return true
}

// Session is one open login of the user.
type Session struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created"`
	Seen      time.Time `json:"seen"`
	UserAgent string    `json:"userAgent"`
	IP        string    `json:"ip"`
	Current   bool      `json:"current"`
}

type sessionHandlers struct {
	db *sql.DB
}

func (h sessionHandlers) list(w http.ResponseWriter, r *http.Request) error {
	return h.write(w, r, v1.UserFrom(r).ID)
}

func (h sessionHandlers) write(w http.ResponseWriter, r *http.Request, userID uint64) error {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, token_hash, created_at, seen_at, user_agent, ip FROM sessions WHERE user_id = ?`, strconv.FormatUint(userID, 10))
	if err != nil {
		return err
	}
	defer rows.Close()
	current, _, _ := login.Session(r)
	items := []Session{}
	var gone []string
	for rows.Next() {
		var s Session
		var hash string
		var created, seen int64
		if err := rows.Scan(&s.ID, &hash, &created, &seen, &s.UserAgent, &s.IP); err != nil {
			return err
		}
		if !login.Alive(hash) {
			gone = append(gone, s.ID)
			continue
		}
		s.Created, s.Seen = time.Unix(created, 0).UTC(), time.Unix(seen, 0).UTC()
		s.Current = hash == current
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	// Logged out or expired sessions leave the list the next time it is read.
	for _, id := range gone {
		if _, err := h.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE id = ?`, id); err != nil {
			return err
		}
	}
	slices.SortFunc(items, func(a, b Session) int { return cmp.Or(b.Seen.Compare(a.Seen), cmp.Compare(a.ID, b.ID)) })
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Session{"items": items})
}

func (h sessionHandlers) end(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	return h.endOne(w, r, v1.UserFrom(r).ID, r.PathValue("id"))
}

func (h sessionHandlers) endOne(w http.ResponseWriter, r *http.Request, userID uint64, id string) error {
	var hash string
	err := h.db.QueryRowContext(r.Context(), `SELECT token_hash FROM sessions WHERE id = ? AND user_id = ?`, id, strconv.FormatUint(userID, 10)).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return v1.Coded(http.StatusNotFound, "noSession", "no such session")
	}
	if err != nil {
		return err
	}
	if err := login.End(hash); err != nil {
		return err
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
