package web

import (
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// sessionLogin connects the accounts of /api/v1 to the sessions and API
// tokens of this login system.
type sessionLogin struct{}

func (sessionLogin) Session(r *http.Request) (hash, ip string, ok bool) {
	token, err := ExtractToken(r)
	if err != nil {
		return "", "", false
	}
	if _, isSession, known := state.HashedTokenOwner(token); !known || !isSession {
		return "", "", false
	}
	return utils.HashSHA256(token), GetRemoteIP(r), true
}

func (sessionLogin) Alive(hash string) bool { return state.SessionAlive(hash) }

func (sessionLogin) End(hash string) error { return endSession(hash) }

func (sessionLogin) EndAll(userID uint64, keep string) (int, error) {
	if next, ok := successor(keep); ok {
		keep = next.hash
	}
	return state.RevokeSessions(userID, keep)
}

// SignedInAt reads the time from the token without checking its signature.
// Only tokens this server minted are registered, and every session starts
// with a new one.
func (sessionLogin) SignedInAt(r *http.Request) (time.Time, bool) {
	token, err := ExtractToken(r)
	if err != nil {
		return time.Time{}, false
	}
	if _, isSession, known := state.HashedTokenOwner(token); !known || !isSession {
		return time.Time{}, false
	}
	var claims sessionClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil || claims.IssuedAt == nil {
		return time.Time{}, false
	}
	if claims.SignedIn != nil {
		return claims.SignedIn.Time, true
	}
	return claims.IssuedAt.Time, true
}

func (sessionLogin) APIToken(r *http.Request) (string, bool) {
	token, err := ExtractToken(r)
	if err != nil {
		return "", false
	}
	if _, isSession, known := state.HashedTokenOwner(token); !known || isSession {
		return "", false
	}
	return token, true
}

// browserSession is how long a session that ends with the browser may run
// at most, for a tab that stays open for days.
const browserSession = 12 * time.Hour

func (sessionLogin) Start(w http.ResponseWriter, r *http.Request, u *users.User, keep time.Duration) error {
	life := keep
	if life <= 0 {
		life = time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)
		if life < browserSession {
			life = browserSession
		}
	}
	token, err := issueSession(u, life, sessionClaims{Stay: keep > 0})
	if err != nil {
		return err
	}
	// Without keep the cookie has no expiry, so the browser drops it when it closes.
	var expires time.Time
	if keep > 0 {
		expires = time.Now().Add(keep)
	}
	SetSessionCookie(w, r, token, expires)
	return nil
}

func (sessionLogin) Stop(w http.ResponseWriter, r *http.Request) error {
	token, err := ExtractToken(r)
	if err == nil {
		if err := endSession(utils.HashSHA256(token)); err != nil {
			return err
		}
	}
	http.SetCookie(w, sessionCookie(r, "", time.Unix(0, 0), -1, http.SameSiteStrictMode))
	return nil
}

// renewAfter is how much of its life a session that stays signed in uses up
// before a visit renews it: a day, or an hour of a session of one day.
func renewAfter(life time.Duration) time.Duration {
	if life < 2*24*time.Hour {
		return life / 24
	}
	return 24 * time.Hour
}

// renewal is the token that replaced a renewed one, kept while the old one
// still works.
type renewal struct {
	token, hash    string
	expires, until time.Time
}

// renewals maps the hash of each token renewed within BearerTokenGrace to
// its successor. Requests still on their way with the old token get the
// same new one, and ending either ends both.
var renewals = struct {
	sync.Mutex
	next map[string]renewal
}{next: map[string]renewal{}}

func successor(hash string) (renewal, bool) {
	renewals.Lock()
	defer renewals.Unlock()
	n, ok := renewals.next[hash]
	return n, ok && time.Now().Before(n.until)
}

// endSession revokes the session token with this hash, with the token it
// replaced or was replaced by.
func endSession(hash string) error {
	hashes := []string{hash}
	renewals.Lock()
	now := time.Now()
	for old, n := range renewals.next {
		if now.After(n.until) {
			continue
		}
		if old == hash {
			hashes = append(hashes, n.hash)
		} else if n.hash == hash {
			hashes = append(hashes, old)
		}
	}
	renewals.Unlock()
	for _, h := range hashes {
		if err := state.RevokeTokenHash(h); err != nil {
			return err
		}
	}
	return nil
}

// Renew gives a session that stays signed in a new token for the time the
// user chose, once renewAfter of its token is used up, so the session ends
// only after that long without a visit.
func (sessionLogin) Renew(w http.ResponseWriter, r *http.Request, u *users.User) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	// A request that brings a token in its header signed in with that one.
	if token, err := ExtractToken(r); err != nil || token != cookie.Value {
		return "", false
	}
	var claims sessionClaims
	if _, _, err := jwt.NewParser().ParseUnverified(cookie.Value, &claims); err != nil || !claims.Stay || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return "", false
	}
	issued := claims.IssuedAt.Time
	if time.Since(issued) < renewAfter(claims.ExpiresAt.Sub(issued)) {
		return "", false
	}

	old := utils.HashSHA256(cookie.Value)
	renewals.Lock()
	defer renewals.Unlock()
	now := time.Now()
	for h, n := range renewals.next {
		if now.After(n.until) {
			delete(renewals.next, h)
		}
	}
	if n, ok := renewals.next[old]; ok {
		SetSessionCookie(w, r, n.token, n.expires)
		return n.hash, true
	}
	keep, err := prefs.StaySignedIn(r.Context(), state.MainDB(), u.ID)
	if err != nil {
		logger.Errorf("renew session: %v", err)
		return "", false
	}
	token, err := issueSession(u, keep, sessionClaims{SignedIn: claims.SignedIn, Stay: true})
	if err != nil {
		logger.Errorf("renew session: %v", err)
		return "", false
	}
	if err := state.RetireSessionToken(cookie.Value); err != nil {
		logger.Errorf("renew session: %v", err)
		if err := state.RevokeToken(token); err != nil {
			logger.Errorf("drop the new session token: %v", err)
		}
		return "", false
	}
	n := renewal{token: token, hash: utils.HashSHA256(token), expires: now.Add(keep), until: now.Add(state.BearerTokenGrace)}
	renewals.next[old] = n
	SetSessionCookie(w, r, n.token, n.expires)
	return n.hash, true
}

func (sessionLogin) ClientIP(r *http.Request) string { return GetRemoteIP(r) }

func (sessionLogin) PasskeySite(r *http.Request) (rpID, host string) {
	return registerRequestOrigin(r), requestHost(r)
}

// terminalGate tells the terminal how a request signed in and where it came
// from.
type terminalGate struct {
	login sessionLogin
	guard *http.CrossOriginProtection
}

func (g terminalGate) SameOrigin(r *http.Request) error { return sameOrigin(g.guard, r) }

func (g terminalGate) Session(r *http.Request) (string, bool) {
	hash, _, ok := g.login.Session(r)
	return hash, ok
}

func (g terminalGate) Alive(hash string) bool { return g.login.Alive(hash) }

func (g terminalGate) APIToken(r *http.Request) bool {
	_, ok := g.login.APIToken(r)
	return ok
}

func (terminalGate) Open() bool { return settings.Config.Auth.Methods.NoAuth }
