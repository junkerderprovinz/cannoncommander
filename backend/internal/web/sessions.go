package web

import (
	"net/http"
	"time"

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

func (sessionLogin) Start(w http.ResponseWriter, r *http.Request, u *users.User) error {
	token, err := mintAndRegisterSessionToken(u)
	if err != nil {
		return err
	}
	expires := time.Now().Add(time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)).Add(sessionCookieBuffer)
	SetSessionCookie(w, r, token, expires)
	return nil
}

func (sessionLogin) ClientIP(r *http.Request) string { return GetRemoteIP(r) }

func (sessionLogin) PasskeySite(r *http.Request) (rpID, host string) {
	registerRequestOrigin(r)
	return deriveRPID(r), requestHost(r)
}
