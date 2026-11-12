//go:build desktop

// Package modules lets Settings fetch the helper programs the desktop app
// brings, ffmpeg, 7-Zip and mutool, when they are missing.
package modules

import (
	_ "embed"
	"errors"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var errNotOffered = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app on Windows and Linux fetches its helper programs")

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("modules", contract)
	r.Handle("GET /modules", get)
	r.Handle("POST /modules", fetch)
}

func app() (desktop.Modules, error) {
	m, ok := desktop.Current().(desktop.Modules)
	if !ok {
		return nil, errNotOffered
	}
	return m, nil
}

// get is GET /modules.
func get(w http.ResponseWriter, r *http.Request) error {
	m, err := app()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, m.ModulesState())
}

// fetch is POST /modules.
func fetch(w http.ResponseWriter, r *http.Request) error {
	m, err := app()
	if err != nil {
		return err
	}
	if err := m.FetchModules(); errors.Is(err, errors.ErrUnsupported) {
		return errNotOffered
	} else if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, m.ModulesState())
}
