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

// Login is what the session list needs from the login system.
type Login interface {
	// Session returns the hash of the session token r carries and the
	// client's address. ok is false for API tokens.
	Session(r *http.Request) (hash, ip string, ok bool)
	// Alive reports whether the session token with this hash still logs in.
	Alive(hash string) bool
	// End revokes the session token with this hash.
	End(hash string) error
}

var login Login

// UseLogin connects the session list to the login system. The server calls
// it once before it starts serving.
func UseLogin(l Login) { login = l }

// seenEvery limits how often a session's last use is written, since every
// request passes through Track.
const seenEvery = time.Minute

var (
	seenMu sync.Mutex
	seen   = map[string]time.Time{}
)

// Track wraps the authenticator of /api/v1 so every session that makes a
// request shows up in the list with its browser, address and last use.
func Track(db *sql.DB, next v1.Authenticator) v1.Authenticator {
	return func(w http.ResponseWriter, r *http.Request) (*users.User, error) {
		u, err := next(w, r)
		if err != nil {
			return u, err
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
			rand.Text(), strconv.FormatUint(u.ID, 10), hash, now, now, ua, ip); err != nil {
			logger.Errorf("record session: %v", err)
		}
		return u, nil
	}
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
