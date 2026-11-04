package web

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// sessionLogin lets the session list of /api/v1 see and end the session
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
