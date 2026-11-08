//go:build !desktop

package fileops

import (
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// templates lists none: a server has no programs of its own to make
// documents for.
func (s *Service) templates(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"templates": []any{}})
}

func (s *Service) fromTemplate(http.ResponseWriter, *http.Request) error {
	return v1.Coded(http.StatusBadRequest, "desktopOnly", "templates belong to the desktop app")
}
