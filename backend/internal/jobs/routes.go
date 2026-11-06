// Package jobs runs copy, move and the other long tasks in the background.
package jobs

import (
	_ "embed"
	"encoding/json"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and, when there is a database, the
// handlers.
func Routes(r *v1.Router) {
	r.Contract("jobs", contract)
	m := For(r.Deps())
	if m == nil {
		return
	}
	h := handlers{m: m}
	r.Handle("GET /jobs", h.list)
	r.Handle("POST /jobs", h.create)
	r.Handle("DELETE /jobs", h.clear)
	r.Handle("GET /jobs/settings", h.settings)
	r.Handle("PUT /jobs/settings", h.putSettings)
	r.Handle("GET /jobs/{id}", h.get)
	r.Handle("PATCH /jobs/{id}", h.patch)
	r.Handle("DELETE /jobs/{id}", h.remove)
	r.Handle("POST /jobs/{id}/pause", h.action(m.Pause))
	r.Handle("POST /jobs/{id}/resume", h.action(m.Resume))
	r.Handle("POST /jobs/{id}/cancel", h.action(m.Cancel))
	r.Handle("POST /jobs/{id}/retry", h.action(m.Retry))
	r.Handle("POST /jobs/{id}/decide", h.decide)
}

type handlers struct {
	m *Manager
}

type jobList struct {
	Jobs []Job `json:"jobs"`
}

func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, jobList{Jobs: h.m.List(ownerFilter(v1.UserFrom(r)))})
}

type createRequest struct {
	Kind          string          `json:"kind"`
	Params        json.RawMessage `json:"params"`
	Cap           int64           `json:"cap"`
	WaitForParity bool            `json:"waitForParity"`
}

func (h handlers) create(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req createRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	k, ok := kindOf(req.Kind)
	if !ok || k.Prepare == nil {
		return v1.Errorf(http.StatusBadRequest, "unknown job kind")
	}
	plan, err := k.Prepare(r.Context(), h.m.deps, u, req.Params)
	if err != nil {
		return err
	}
	j, err := h.m.Submit(u.Username, req.Kind, plan, req.Cap, req.WaitForParity)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, j)
}

type cleared struct {
	Removed int `json:"removed"`
}

func (h handlers) clear(w http.ResponseWriter, r *http.Request) error {
	n := h.m.ClearFinished(ownerFilter(v1.UserFrom(r)))
	return v1.WriteJSON(w, r, http.StatusOK, cleared{Removed: n})
}

func (h handlers) settings(w http.ResponseWriter, r *http.Request) error {
	return v1.WriteJSON(w, r, http.StatusOK, h.m.Settings())
}

func (h handlers) putSettings(w http.ResponseWriter, r *http.Request) error {
	var s Settings
	if err := v1.DecodeJSON(r, &s); err != nil {
		return err
	}
	out, err := h.m.SetSettings(s)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) get(w http.ResponseWriter, r *http.Request) error {
	j, err := h.find(r)
	if err != nil {
		return err
	}
	j, _ = h.m.Get(j.ID, true)
	return v1.WriteJSON(w, r, http.StatusOK, j)
}

type patchRequest struct {
	Position *int   `json:"position"`
	Cap      *int64 `json:"cap"`
}

func (h handlers) patch(w http.ResponseWriter, r *http.Request) error {
	j, err := h.find(r)
	if err != nil {
		return err
	}
	var req patchRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Cap != nil {
		if j, err = h.m.SetCap(j.ID, *req.Cap); err != nil {
			return err
		}
	}
	if req.Position != nil {
		if j, err = h.m.Move(j.ID, *req.Position); err != nil {
			return err
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, j)
}

func (h handlers) remove(w http.ResponseWriter, r *http.Request) error {
	j, err := h.find(r)
	if err != nil {
		return err
	}
	if err := h.m.Remove(j.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) action(f func(string) (Job, error)) v1.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		j, err := h.find(r)
		if err != nil {
			return err
		}
		if j, err = f(j.ID); err != nil {
			return err
		}
		return v1.WriteJSON(w, r, http.StatusOK, j)
	}
}

type decideRequest struct {
	Key    string `json:"key"`
	Choice string `json:"choice"`
	All    bool   `json:"all"`
}

func (h handlers) decide(w http.ResponseWriter, r *http.Request) error {
	j, err := h.find(r)
	if err != nil {
		return err
	}
	var req decideRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if j, err = h.m.Decide(j.ID, req.Key, req.Choice, req.All); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, j)
}

// find returns the job in the path. Other users' jobs do not exist for a
// user who is not an admin.
func (h handlers) find(r *http.Request) (Job, error) {
	j, ok := h.m.Get(r.PathValue("id"), false)
	if !ok || !mayTouch(v1.UserFrom(r), j) {
		return Job{}, v1.Coded(http.StatusNotFound, "noJob", "no such job")
	}
	return j, nil
}

func mayTouch(u *users.User, j Job) bool {
	return u != nil && (u.Permissions.Admin || u.Username == j.Owner)
}

// ownerFilter is the owner whose jobs u sees, empty for all of them.
func ownerFilter(u *users.User) string {
	if u.Permissions.Admin {
		return ""
	}
	return u.Username
}
