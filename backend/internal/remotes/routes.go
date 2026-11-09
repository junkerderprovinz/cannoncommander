// Package remotes mounts rclone remotes under /remotes. rclone runs as
// rclone rcd, a child process with its own rclone.conf, and is driven over
// its remote control API on a Unix socket.
package remotes

import (
	"context"
	_ "embed"
	"errors"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers, marks mounted folders in
// listings and the tree, and mounts the saved remotes once rclone is up.
func Routes(r *v1.Router) {
	r.Contract("remotes", contract)
	d := r.Deps()
	svc, _ := v1.Shared(d, "remotes", func() (*Service, error) {
		return newService(ConfigFromEnv(), store{d.DB}, mountingOf(d)), nil
	})
	bind(r, svc)
	fsapi.SetRemotes(svc.labelAt)
	system.RegisterSection(svc.exportSection())
	if d.DB != nil {
		system.OnShutdown(svc.Stop)
		go svc.boot(context.Background())
	}
}

func mountingOf(d v1.Deps) func(context.Context) (system.Mounting, error) {
	return func(ctx context.Context) (system.Mounting, error) {
		a, err := system.Settings(ctx, d)
		return a.Remotes, err
	}
}

func bind(r *v1.Router, s *Service) {
	h := handlers{s}
	r.Handle("GET /remotes", h.list)
	r.Handle("POST /remotes", h.create)
	r.Handle("GET /remotes/types", h.types)
	r.Handle("POST /remotes/test", h.test)
	r.Handle("POST /remotes/oauth", h.oauth)
	r.Handle("GET /remotes/{name}", h.get)
	r.Handle("PUT /remotes/{name}", h.update)
	r.Handle("DELETE /remotes/{name}", h.delete)
	r.Handle("POST /remotes/{name}/mount", h.mount)
	r.Handle("POST /remotes/{name}/unmount", h.unmount)
}

type handlers struct{ s *Service }

// Catalog is what the add dialog picks from: the products by name, and the
// rclone backends no product stands for.
type Catalog struct {
	Types     []Type     `json:"types"`
	Providers []Provider `json:"providers"`
	Unlisted  []string   `json:"unlisted"`
}

func admin(r *http.Request) bool {
	u := v1.UserFrom(r)
	return u != nil && u.Permissions.Admin
}

// problem gives rclone's errors their status. What rclone says about a
// remote is about the input, so it goes back as it is.
func problem(err error) error {
	var rc *rcError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errUnavailable):
		return v1.Errorf(http.StatusServiceUnavailable, errUnavailable.Error())
	case errors.Is(err, errNotRunning):
		return v1.Errorf(http.StatusServiceUnavailable, "rclone is not running yet, please try again in a moment")
	case errors.As(err, &rc):
		return v1.Errorf(http.StatusBadRequest, "rclone: "+rc.Msg)
	}
	return err
}

func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	out, err := h.s.List(r.Context(), admin(r))
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) get(w http.ResponseWriter, r *http.Request) error {
	out, err := h.s.Get(r.Context(), r.PathValue("name"), admin(r))
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) types(w http.ResponseWriter, r *http.Request) error {
	out, err := h.s.Types(r.Context(), r.URL.Query().Get("advanced") == "true")
	if err != nil {
		return problem(err)
	}
	provs, unlisted := providersFor(out)
	return v1.WriteJSON(w, r, http.StatusOK, Catalog{Types: out, Providers: provs, Unlisted: unlisted})
}

func (h handlers) create(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.s.Create(r.Context(), in)
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusCreated, out)
}

func (h handlers) update(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.s.Update(r.Context(), r.PathValue("name"), in)
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) delete(w http.ResponseWriter, r *http.Request) error {
	if err := h.s.Delete(r.Context(), r.PathValue("name")); err != nil {
		return problem(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) mount(w http.ResponseWriter, r *http.Request) error {
	out, err := h.s.Mount(r.Context(), r.PathValue("name"))
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) unmount(w http.ResponseWriter, r *http.Request) error {
	out, err := h.s.Unmount(r.Context(), r.PathValue("name"))
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) test(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.s.Test(r.Context(), in)
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) oauth(w http.ResponseWriter, r *http.Request) error {
	var in OAuthInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.s.OAuth(r.Context(), in)
	if err != nil {
		return problem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}
