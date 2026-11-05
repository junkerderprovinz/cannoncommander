package v1

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// Problem is an RFC 9457 error body. Handlers return it as an error.
type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// Errorf returns a problem with the given status and detail.
func Errorf(status int, detail string) *Problem {
	return &Problem{Title: http.StatusText(status), Status: status, Detail: detail}
}

// ProblemFor turns any error into a problem. Errors that are not a Problem
// and not one of the known file system errors become a 500 without detail,
// so internals do not leak to the client.
func ProblemFor(err error) *Problem {
	var p *Problem
	switch {
	case errors.As(err, &p):
		return p
	case errors.Is(err, ccfs.ErrBadPath):
		return Errorf(http.StatusBadRequest, "invalid path")
	case errors.Is(err, ccfs.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return Errorf(http.StatusNotFound, "path not found")
	case errors.Is(err, ccfs.ErrProtected):
		return Errorf(http.StatusForbidden, "a rule protects something in this folder")
	case errors.Is(err, ccfs.ErrReadOnly):
		return Errorf(http.StatusForbidden, "path is read only")
	case errors.Is(err, ccfs.ErrForbidden), errors.Is(err, ccfs.ErrEscape), errors.Is(err, fs.ErrPermission):
		return Errorf(http.StatusForbidden, "access denied")
	}
	return &Problem{Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError}
}

// WriteError answers with the problem for err.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	// Packages keep common problems in variables, so the instance goes on a copy.
	p := *ProblemFor(err)
	if p.Status >= 500 && p.Status != http.StatusNotImplemented {
		logger.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
	}
	if p.Instance == "" {
		p.Instance = r.URL.Path
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
