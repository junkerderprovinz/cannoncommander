// Package unraid reads the array state from emhttp and talks to Unraid.
package unraid

import (
	"context"
	_ "embed"
	"errors"
	"net/http"
	"os"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers. On a server with emhttp
// files it also starts the monitor for spin and parity events.
func Routes(r *v1.Router) {
	r.Contract("unraid", contract)
	svc := Default()
	bind(r, svc)
	if h := r.Deps().Hub; h != nil {
		if st, err := svc.State(); err == nil && st.Available {
			go svc.Watch(context.Background(), h)
		}
	}
}

func bind(r *v1.Router, svc *Service) {
	h := handlers{svc: svc, fs: r.Deps().FS}
	r.Handle("GET /unraid/state", h.state)
	r.Handle("GET /unraid/where", h.where)
	r.Handle("POST /unraid/transfer", h.transfer)
	r.Handle("POST /unraid/spindown", h.spindown)
	r.Handle("POST /unraid/gather/plan", h.gather)
	r.Handle("POST /unraid/fixperms/scan", h.fixpermsScan)
	r.Handle("POST /unraid/notify/test", h.notifyTest)
}

type handlers struct {
	svc *Service
	fs  *ccfs.Resolver
}

func (h handlers) state(w http.ResponseWriter, r *http.Request) error {
	st, err := h.svc.State()
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, st)
}

func (h handlers) where(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, err := h.fs.Check(u, r.URL.Query().Get("path"), ccfs.View)
	if err != nil {
		return err
	}
	dir := loc.Virtual()
	if !dir && !loc.InArchive() {
		info, err := os.Stat(loc.Real)
		if err != nil {
			return err
		}
		dir = info.IsDir()
	}
	out, err := h.svc.Where(r.Context(), loc.Path, dir)
	if err != nil {
		return err
	}
	if out.Links != nil {
		out.Links.Paths = h.visible(u, out.Links.Paths)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) transfer(w http.ResponseWriter, r *http.Request) error {
	var t Transfer
	if err := v1.DecodeJSON(r, &t); err != nil {
		return err
	}
	switch t.Kind {
	case "copy", "move", "link":
	default:
		return v1.Errorf(http.StatusBadRequest, "kind must be copy, move or link")
	}
	if len(t.Paths) == 0 || t.Dest == "" {
		return v1.Errorf(http.StatusBadRequest, "paths and dest are required")
	}
	u := v1.UserFrom(r)
	dest, err := h.fs.Check(u, t.Dest, ccfs.View)
	if err != nil {
		return err
	}
	t.Dest = dest.Path
	for i, p := range t.Paths {
		loc, err := h.fs.Check(u, p, ccfs.View)
		if err != nil {
			return err
		}
		t.Paths[i] = loc.Path
	}
	rep, err := h.svc.CheckTransfer(r.Context(), t)
	if err != nil {
		return err
	}
	rep.Keep(func(p string) bool { return h.sees(u, p) })
	return v1.WriteJSON(w, r, http.StatusOK, rep)
}

func (h handlers) spindown(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Disks []string `json:"disks"`
	}
	if r.ContentLength != 0 {
		if err := v1.DecodeJSON(r, &req); err != nil {
			return err
		}
	}
	done, err := h.svc.Spindown(r.Context(), req.Disks)
	if err != nil {
		return writeProblem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]string{"disks": done})
}

func (h handlers) gather(w http.ResponseWriter, r *http.Request) error {
	var req GatherRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	loc, err := h.fs.Check(v1.UserFrom(r), req.Path, ccfs.View)
	if err != nil {
		return err
	}
	req.Path = loc.Path
	plan, err := h.svc.PlanGather(r.Context(), req)
	if err != nil {
		return writeProblem(err)
	}
	return v1.WriteJSON(w, r, http.StatusOK, plan)
}

func (h handlers) fixpermsScan(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Path string `json:"path"`
	}
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	loc, err := h.fs.Check(v1.UserFrom(r), req.Path, ccfs.View)
	if err != nil {
		return err
	}
	if loc.Virtual() {
		return v1.Errorf(http.StatusBadRequest, "rights are only set below /mnt")
	}
	scan, err := h.svc.ScanPerms(r.Context(), loc)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, scan)
}

func (h handlers) notifyTest(w http.ResponseWriter, r *http.Request) error {
	n := Notice{Subject: "CannonCommander", Description: "So sieht die Meldung aus, wenn eine Aufgabe fertig ist"}
	if r.ContentLength != 0 {
		var req Notice
		if err := v1.DecodeJSON(r, &req); err != nil {
			return err
		}
		if req.Subject != "" {
			n.Subject = req.Subject
		}
		if req.Description != "" {
			n.Description = req.Description
		}
	}
	if err := h.svc.Notify(n); err != nil {
		return writeProblem(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// writeProblem gives the errors of this package their status codes.
func writeProblem(err error) error {
	switch {
	case errors.Is(err, ErrWriteOff):
		return v1.Errorf(http.StatusForbidden, "write access to Unraid is switched off")
	case errors.Is(err, ErrUnknownDisk), errors.Is(err, ErrNotUserFolder), errors.Is(err, errBadMode):
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	return err
}

func (h handlers) sees(u *users.User, p string) bool {
	_, err := h.fs.Check(u, p, ccfs.View)
	return err == nil
}

func (h handlers) visible(u *users.User, paths []string) []string {
	out := []string{}
	for _, p := range paths {
		if h.sees(u, p) {
			out = append(out, p)
		}
	}
	return out
}
