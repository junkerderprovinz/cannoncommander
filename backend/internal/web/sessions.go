package web

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
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

func (sessionLogin) End(hash string) error { return state.RevokeTokenHash(hash) }

func (sessionLogin) EndAll(userID uint64, keep string) (int, error) {
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
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil || claims.IssuedAt == nil {
		return time.Time{}, false
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
	token, err := mintSessionToken(u, life)
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
		if err := state.RevokeToken(token); err != nil {
			return err
		}
	}
	http.SetCookie(w, sessionCookie(r, "", time.Unix(0, 0), -1, http.SameSiteStrictMode))
	return nil
}

func (sessionLogin) ClientIP(r *http.Request) string { return GetRemoteIP(r) }

func (sessionLogin) PasskeySite(r *http.Request) (rpID, host string) {
	registerRequestOrigin(r)
	return deriveRPID(r), requestHost(r)
}
