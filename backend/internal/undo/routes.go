// Package undo keeps the steps that Ctrl+Z can take back.
package undo

import (
	_ "embed"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and, when there is a database, the
// handlers.
func Routes(r *v1.Router) {
	r.Contract("undo", contract)
	l := For(r.Deps())
	if l == nil {
		return
	}
	r.Handle("GET /undo", l.list)
	r.Handle("POST /undo", l.take)
}

type stepList struct {
	Steps []Step `json:"steps"`
}

func (l *Log) list(w http.ResponseWriter, r *http.Request) error {
	steps, err := l.List(v1.UserFrom(r).Username)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, stepList{Steps: steps})
}

type takeRequest struct {
	ID string `json:"id"`
}

type taken struct {
	Step Step      `json:"step"`
	Job  *jobs.Job `json:"job,omitempty"`
}

func (l *Log) take(w http.ResponseWriter, r *http.Request) error {
	var req takeRequest
	if r.ContentLength != 0 {
		if err := v1.DecodeJSON(r, &req); err != nil {
			return err
		}
	}
	s, j, err := l.Take(r.Context(), v1.UserFrom(r), req.ID)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if j != nil {
		status = http.StatusAccepted
	}
	return v1.WriteJSON(w, r, status, taken{Step: s, Job: j})
}
