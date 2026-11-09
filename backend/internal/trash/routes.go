// Package trash moves deleted files into a trash folder per share or volume.
package trash

import (
	_ "embed"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and, when there is a database, the
// handlers.
func Routes(r *v1.Router) {
	r.Contract("trash", contract)
	s, m := For(r.Deps()), jobs.For(r.Deps())
	if s == nil || m == nil {
		return
	}
	h := handlers{s: s, m: m}
	r.Handle("GET /trash", h.list)
	r.Handle("POST /trash/restore", h.restore)
	r.Handle("POST /trash/empty", h.empty)
	r.Handle("DELETE /trash/{id}", h.drop)
	r.Handle("GET /trash/settings", h.settings)
	r.Handle("PUT /trash/settings", h.putSettings)
}

type handlers struct {
	s *Store
	m *jobs.Manager
}

type listing struct {
	Items []Item `json:"items"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Days  int    `json:"days"`
	// System says that the list is the trash of the operating system, which
	// decides itself how long entries stay.
	System bool `json:"system,omitempty"`
}

func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	items, err := h.s.List(ownerFilter(v1.UserFrom(r)))
	if err != nil {
		return err
	}
	out := listing{Items: items, Count: len(items), Days: h.s.Days(), System: h.s.System()}
	for _, it := range items {
		out.Bytes += it.Size
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

type idsRequest struct {
	IDs []string `json:"ids"`
}

func (h handlers) restore(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req idsRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.IDs) == 0 {
		return v1.Errorf(http.StatusBadRequest, "ids are required")
	}
	entries, err := h.pick(u, req.IDs)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := h.s.mayRestore(u, e); err != nil {
			return err
		}
	}
	return h.submit(w, r, KindRestore, entries)
}

func (h handlers) empty(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req idsRequest
	if r.ContentLength != 0 {
		if err := v1.DecodeJSON(r, &req); err != nil {
			return err
		}
	}
	var entries []Item
	var err error
	if len(req.IDs) == 0 {
		entries, err = h.s.List(ownerFilter(u))
	} else {
		entries, err = h.pick(u, req.IDs)
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return v1.Coded(http.StatusBadRequest, "trashEmpty", "the trash is empty")
	}
	return h.submit(w, r, KindEmpty, entries)
}

func (h handlers) drop(w http.ResponseWriter, r *http.Request) error {
	entries, err := h.pick(v1.UserFrom(r), []string{r.PathValue("id")})
	if err != nil {
		return err
	}
	return h.submit(w, r, KindEmpty, entries)
}

func (h handlers) submit(w http.ResponseWriter, r *http.Request, kind string, entries []Item) error {
	p := plan(entries)
	if a, ok := v1.AgentFrom(r); ok {
		p.Via = a.Name
	}
	j, err := h.m.Submit(v1.UserFrom(r).Username, kind, p, 0, false)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, j)
}

// pick returns the entries with the given ids. An entry of somebody else
// does not exist for a user who is not an admin. The system's trash has no
// owners.
func (h handlers) pick(u *users.User, ids []string) ([]Item, error) {
	found, err := h.s.getAll(ids)
	if err != nil {
		return nil, err
	}
	var out []Item
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		it, ok := found[id]
		if !ok || !u.Permissions.Admin && !systemID(id) && it.By != u.Username {
			return nil, ErrNotFound
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, it)
		}
	}
	return out, nil
}

// Settings are the trash settings shared by all users.
type Settings struct {
	Days int `json:"days"`
}

func (h handlers) settings(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, Settings{Days: h.s.Days()})
}

func (h handlers) putSettings(w http.ResponseWriter, r *http.Request) error {
	var s Settings
	if err := v1.DecodeJSON(r, &s); err != nil {
		return err
	}
	if err := h.s.SetDays(s.Days); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, Settings{Days: h.s.Days()})
}

func ownerFilter(u *users.User) string {
	if u.Permissions.Admin {
		return ""
	}
	return u.Username
}
