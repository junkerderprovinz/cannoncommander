//go:build desktop

// Package filemanager is the switch that makes the desktop app the
// computer's file manager, so folders other programs open come to it.
package filemanager

import (
	_ "embed"
	"errors"
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var (
	errNotOffered = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app on Windows and Linux takes the place of the file manager")
	errNotSet     = v1.Coded(http.StatusInternalServerError, "fileManagerNotSet", "the computer did not take the change")
)

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("filemanager", contract)
	r.Handle("GET /filemanager", get)
	r.Handle("PUT /filemanager", put)
}

type state struct {
	On bool `json:"on"`
}

func app() (desktop.FileManager, error) {
	fm, ok := desktop.Current().(desktop.FileManager)
	if !ok {
		return nil, errNotOffered
	}
	return fm, nil
}

// get is GET /filemanager.
func get(w http.ResponseWriter, r *http.Request) error {
	fm, err := app()
	if err != nil {
		return err
	}
	on, err := fm.IsFileManager()
	if errors.Is(err, errors.ErrUnsupported) {
		return errNotOffered
	}
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, state{On: on})
}

// put is PUT /filemanager.
func put(w http.ResponseWriter, r *http.Request) error {
	fm, err := app()
	if err != nil {
		return err
	}
	var in struct {
		On    bool          `json:"on"`
		Words desktop.Words `json:"words"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	err = fm.SetFileManager(in.On, in.Words)
	if errors.Is(err, errors.ErrUnsupported) {
		return errNotOffered
	}
	if err != nil {
		logger.Errorf("file manager switch to %v: %v", in.On, err)
		return errNotSet
	}
	on, err := fm.IsFileManager()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, state{On: on})
}
