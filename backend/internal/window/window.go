//go:build desktop

// Package window lets the page work the desktop app's main window from the
// title bar it draws itself on Windows and Linux.
package window

import (
	_ "embed"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var errNotOffered = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app has a window")

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("window", contract)
	r.Handle("GET /window", get)
	r.Handle("PUT /window", put)
	r.Handle("POST /window/minimise", minimise)
	r.Handle("POST /window/close", closeWindow)
}

type state struct {
	Maximised bool `json:"maximised"`
}

func app() (desktop.Window, error) {
	w, ok := desktop.Current().(desktop.Window)
	if !ok {
		return nil, errNotOffered
	}
	return w, nil
}

// get is GET /window.
func get(w http.ResponseWriter, r *http.Request) error {
	win, err := app()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, state{Maximised: win.WindowMaximised()})
}

// put is PUT /window.
func put(w http.ResponseWriter, r *http.Request) error {
	win, err := app()
	if err != nil {
		return err
	}
	var in state
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	win.MaximiseWindow(in.Maximised)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// minimise is POST /window/minimise.
func minimise(w http.ResponseWriter, r *http.Request) error {
	win, err := app()
	if err != nil {
		return err
	}
	win.MinimiseWindow()
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// closeWindow is POST /window/close.
func closeWindow(w http.ResponseWriter, r *http.Request) error {
	win, err := app()
	if err != nil {
		return err
	}
	win.CloseWindow()
	w.WriteHeader(http.StatusNoContent)
	return nil
}
