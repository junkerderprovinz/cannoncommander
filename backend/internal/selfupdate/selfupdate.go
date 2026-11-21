//go:build desktop

// Package selfupdate lets the Version card start the desktop app's own update
// and follow it, for "Update now".
package selfupdate

import (
	_ "embed"
	"errors"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var errNotOffered = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app outside a Flatpak updates itself")

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("selfupdate", contract)
	r.Handle("GET /admin/app/update", get)
	r.Handle("POST /admin/app/update", start)
}

func app() (desktop.Updates, error) {
	u, ok := desktop.Current().(desktop.Updates)
	if !ok {
		return nil, errNotOffered
	}
	return u, nil
}

// get is GET /admin/app/update.
func get(w http.ResponseWriter, r *http.Request) error {
	u, err := app()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, u.UpdateState())
}

// start is POST /admin/app/update.
func start(w http.ResponseWriter, r *http.Request) error {
	u, err := app()
	if err != nil {
		return err
	}
	if err := u.UpdateNow(); errors.Is(err, errors.ErrUnsupported) {
		return errNotOffered
	} else if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, u.UpdateState())
}
