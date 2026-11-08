//go:build desktop

package fileops

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/templates"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// templates lists what New offers: the documents the computer's programs
// make.
func (s *Service) templates(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"templates": templates.List()})
}

type templateRequest struct {
	Path     string `json:"path"`
	Template string `json:"template"`
}

// fromTemplate makes a file from a template. Undo takes it back as long as
// it holds what the template gave it.
func (s *Service) fromTemplate(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req templateRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	loc, err := s.newEntry(u, req.Path)
	if err != nil {
		return err
	}
	if err := templates.Make(req.Template, loc.Real); err != nil {
		switch {
		case errors.Is(err, fs.ErrExist):
			return errExists
		case errors.Is(err, templates.ErrUnknown):
			return v1.Coded(http.StatusNotFound, "noTemplate", "there is no such template")
		}
		return err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	fschange.Created(loc.Path)
	out := Changed{Path: loc.Path, Created: true}
	out.Undo = s.record(u, undo.Step{Kind: StepTouch, Name: path.Base(loc.Path), Count: 1, Path: path.Dir(loc.Path)},
		entryData{Path: loc.Path, MTime: info.ModTime(), Size: info.Size()})
	return v1.WriteJSON(w, r, http.StatusCreated, out)
}
