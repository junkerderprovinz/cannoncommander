package web

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/dav"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

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
	if !ok {
		return dav.Identity{}, accounts.ErrBasicRefused
	}
	in, err := accounts.BasicLogin(r, name, password)
	if err != nil {
		return dav.Identity{}, err
	}
	SetUserInResponseWriter(w, in.User)
	return dav.Identity{User: in.User, ReadOnly: in.ReadOnly}, nil
}
