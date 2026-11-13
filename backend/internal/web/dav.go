package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/dav"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

var errNotADavToken = errors.New("not an API token")

// mountDAV serves WebDAV at /dav. The switch in the settings is read for every
// request, so nothing here depends on how the server started.
func mountDAV(router *http.ServeMux, deps v1.Deps) {
	prefix := settings.Config.Http.BaseURL + "dav"
	h := dav.New(deps, dav.Options{
		Prefix:  prefix,
		Enabled: func() bool { return !settings.Config.Http.DisableWebDAV },
		SignIn:  davSignIn,
	})
	router.Handle(prefix, h)
	router.Handle(prefix+"/", h)
}

// davSignIn finds the user from the Basic credentials alone. The session
// cookie and the other ways into /api/v1 do not count here, or any page could
// drive the server through the browser of someone signed in.
func davSignIn(w http.ResponseWriter, r *http.Request) (dav.Identity, error) {
	if state.OpenAccess() {
		u, err := state.ResolveNoAuthUser()
		if err != nil {
			return dav.Identity{}, err
		}
		SetUserInResponseWriter(w, &u)
		return dav.Identity{User: &u}, nil
	}
	name, password, ok := r.BasicAuth()
	if !ok || password == "" {
		return dav.Identity{}, accounts.ErrBasicRefused
	}
	// A valid token goes first: it cannot be guessed, so somebody guessing the
	// password of the same account does not have to stop it.
	id, err := davToken(password)
	if err == nil {
		accounts.TokenUsed(r.Context(), state.MainDB(), id.User, password)
	} else {
		var u *users.User
		u, err = accounts.BasicLogin(state.MainDB(), r, name, password)
		if err != nil {
			return dav.Identity{}, err
		}
		id = dav.Identity{User: u}
	}
	SetUserInResponseWriter(w, id.User)
	return id, nil
}

// davToken returns who a named API token stands for, with the rights the token
// leaves. A session token is not one, it is only good in its cookie.
func davToken(raw string) (dav.Identity, error) {
	if strings.Count(raw, ".") != 2 {
		return dav.Identity{}, errNotADavToken
	}
	var claims users.AuthToken
	token, err := auth.ParseToken(raw, &claims)
	if err != nil || !token.Valid || state.IsTokenRevoked(raw) {
		return dav.Identity{}, errNotADavToken
	}
	if _, isSession, known := state.HashedTokenOwner(raw); !known || isSession {
		return dav.Identity{}, errNotADavToken
	}
	u, err := resolveBearerTokenUser(raw)
	if err != nil {
		return dav.Identity{}, errNotADavToken
	}
	name, _ := state.TokenNameForRawToken(u, raw)
	return dav.Identity{User: u, ReadOnly: u.Tokens[name].ReadOnly}, nil
}
