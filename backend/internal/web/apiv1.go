package web

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/api"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// eventHub carries the /api/v1 events. It lives as long as the process.
var eventHub = hub.New(hub.Options{})

// mountAPIv1 serves /api/v1 next to the older /api routes. The more
// specific prefix wins in the ServeMux.
func mountAPIv1(router *http.ServeMux) error {
	var gate ccfs.AccessGate
	if store := state.Default(); store != nil {
		gate = store
	}
	resolver, err := ccfs.FromSettings(&settings.Config, gate)
	if err != nil {
		return err
	}
	deps := v1.Deps{FS: resolver, Hub: eventHub, DB: state.MainDB()}
	accounts.UseLogin(sessionLogin{})
	h, err := api.NewRouter(deps, accounts.Track(deps.DB, authenticateV1)).Handler()
	if err != nil {
		return err
	}
	prefix := settings.Config.Http.BaseURL + "api/v1"
	router.Handle(prefix+"/", http.StripPrefix(prefix, h))
	return nil
}

// authenticateV1 runs the same login checks as the older API, so a session
// cookie, an API token and proxy or JWT logins all work for /api/v1.
func authenticateV1(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	d := &requestContext{Ctx: r.Context()}
	var user *users.User
	status, err := withUserHelper(func(_ http.ResponseWriter, _ *http.Request, d *requestContext) (int, error) {
		user = d.User
		return 0, nil
	})(w, r, d)
	if user != nil {
		return user, nil
	}
	switch status {
	case http.StatusUnauthorized, 0:
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	case http.StatusForbidden:
		return nil, v1.Errorf(http.StatusForbidden, "access denied")
	}
	return nil, err
}
