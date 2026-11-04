package accounts

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
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
	// APIToken returns the API token r logs in with. ok is false for
	// sessions and every other kind of login.
	APIToken(r *http.Request) (raw string, ok bool)
	// Start logs u in on this client with a new session.
	Start(w http.ResponseWriter, r *http.Request, u *users.User) error
	// ClientIP returns the address of the client behind any proxy.
	ClientIP(r *http.Request) string
	// PasskeySite readies passkeys for the address r came in on and returns
	// the relying party id and the host as the browser shows it.
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
// tokens remember when they were used, and a read only token gets no
// further than reading.
func Track(db *sql.DB, next v1.Authenticator) v1.Authenticator {
	return func(w http.ResponseWriter, r *http.Request) (*users.User, error) {
		u, err := next(w, r)
		if err != nil {
			return u, err
		}
		if raw, ok := login.APIToken(r); ok {
			name, _ := users.TokenNameByRaw(u.Tokens, raw)
			if u.Tokens[name].ReadOnly && !safe(r.Method) {
				return nil, v1.Errorf(http.StatusForbidden, "this API token may only read")
			}
			hash := utils.HashSHA256(raw)
			if due("token:" + hash) {
				now := time.Now().Unix()
				// Like the session list, this is bookkeeping that must not fail the request.
				if _, err := db.ExecContext(r.Context(), `INSERT INTO token_use (token_hash, user_id, seen_at) VALUES (?, ?, ?)
					ON CONFLICT (token_hash) DO UPDATE SET seen_at = excluded.seen_at`, hash, idOf(u), now); err != nil {
					logger.Errorf("record token use: %v", err)
				}
			}
			return u, nil
		}
		hash, ip, ok := login.Session(r)
		if !ok || !due(hash) {
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

func safe(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// needSession refuses changes to logins and credentials that come with an
// API token, so a token can never mint a stronger one or lock its owner out.
func needSession(r *http.Request) error {
	if _, ok := login.APIToken(r); ok {
		return v1.Errorf(http.StatusForbidden, "sign in to change this, an API token cannot")
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

// Rotated keeps a session in the list when the login system swaps its
// token for a fresh one.
func Rotated(ctx context.Context, db *sql.DB, oldHash, newHash string) error {
	_, err := db.ExecContext(ctx, `UPDATE sessions SET token_hash = ? WHERE token_hash = ?`, newHash, oldHash)
	return err
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
	uid := strconv.FormatUint(v1.UserFrom(r).ID, 10)
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, token_hash, created_at, seen_at, user_agent, ip FROM sessions WHERE user_id = ?`, uid)
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
	uid := strconv.FormatUint(v1.UserFrom(r).ID, 10)
	var hash string
	err := h.db.QueryRowContext(r.Context(), `SELECT token_hash FROM sessions WHERE id = ? AND user_id = ?`, r.PathValue("id"), uid).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return v1.Errorf(http.StatusNotFound, "no such session")
	}
	if err != nil {
		return err
	}
	if err := login.End(hash); err != nil {
		return err
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE id = ?`, r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
