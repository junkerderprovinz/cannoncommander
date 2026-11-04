// Package prefs keeps per user settings, bookmarks, profiles, rename
// presets, saved searches and the folder history.
package prefs

import (
	"database/sql"
	_ "embed"
	"net/http"
	"strconv"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and its handlers. Saved searches are per
// user data like the rest, so they are served here although their paths
// belong to the search contract, which has to be registered first.
func Routes(r *v1.Router) {
	r.Contract("prefs", contract)
	d := r.Deps()
	h := handlers{db: d.DB, hub: d.Hub, fs: d.FS}
	r.Handle("GET /me/prefs", h.getSettings)
	r.Handle("PUT /me/prefs", h.putSettings)
	r.Handle("GET /me/bookmarks", h.getBookmarks)
	r.Handle("PUT /me/bookmarks", h.putBookmarks)
	r.Handle("GET /me/profiles", h.listProfiles)
	r.Handle("POST /me/profiles", h.createProfile)
	r.Handle("PUT /me/profiles/{id}", h.updateProfile)
	r.Handle("DELETE /me/profiles/{id}", h.deleteProfile)
	r.Handle("GET /me/rename-presets", h.getPresets)
	r.Handle("PUT /me/rename-presets", h.putPresets)
	r.Handle("GET /me/frequent", h.getFrequent)
	r.Handle("POST /me/frequent", h.visit)
	r.Handle("GET /search/saved", h.listSearches)
	r.Handle("POST /search/saved", h.createSearch)
	r.Handle("PUT /search/saved/{id}", h.updateSearch)
	r.Handle("DELETE /search/saved/{id}", h.deleteSearch)
}

type handlers struct {
	db  *sql.DB
	hub *hub.Hub
	fs  *ccfs.Resolver
}

// Change is the payload of prefs.changed. Kind names what changed:
// settings, bookmarks, profiles, renamePresets or savedSearches. Device is
// set when settings were saved from one device.
type Change struct {
	Kind   string `json:"kind"`
	Device string `json:"device,omitempty"`
}

// changed tells the user's other tabs and devices to reload.
func (h handlers) changed(r *http.Request, c Change) {
	_, _ = h.hub.PublishJSON(EventChanged, "", c, v1.UserFrom(r).Username)
}

func userID(r *http.Request) string {
	return strconv.FormatUint(v1.UserFrom(r).ID, 10)
}

func idParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, noEntry()
	}
	return id, nil
}

func noContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}
