package v1

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"syscall"

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
	// Code names the sentence in Detail, so an interface can say it in its
	// own language and fill in Params. Detail stays for every other client.
	Code   string         `json:"code,omitempty"`
	Params map[string]any `json:"params,omitempty"`
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

// Coded returns a problem whose detail an interface can translate by code.
func Coded(status int, code, detail string) *Problem {
	return &Problem{Title: http.StatusText(status), Status: status, Detail: detail, Code: code}
}

// With returns a copy of p that carries one more parameter, so a problem kept
// in a variable stays as it is.
func (p *Problem) With(name string, value any) *Problem {
	c := *p
	c.Params = make(map[string]any, len(p.Params)+1)
	for k, v := range p.Params {
		c.Params[k] = v
	}
	c.Params[name] = value
	return &c
}

// CodeOf names what went wrong in err the way a problem would, for errors that
// reach a client as text, such as the reason a job failed. It also names the
// file system errors a copy or a delete runs into, which carry no code of their own.
func CodeOf(err error) (string, map[string]any) {
	var p *Problem
	var errno syscall.Errno
	switch {
	case errors.As(err, &p):
		return p.Code, p.Params
	case errors.Is(err, ccfs.ErrInCloud):
		return "inCloud", nil
	case errors.As(err, &errno):
		switch errno {
		case syscall.ENOSPC:
			return "noSpace", nil
		case syscall.EACCES, syscall.EPERM:
			return "denied", nil
		case syscall.EROFS:
			return "readOnly", nil
		case syscall.ENOENT:
			return "notFound", nil
		case syscall.EIO:
			return "ioError", nil
		case syscall.ENAMETOOLONG:
			return "nameTooLong", nil
		}
	}
	// Windows says ERROR_ACCESS_DENIED, which is no EACCES.
	if errors.Is(err, fs.ErrPermission) {
		return "denied", nil
	}
	return "", nil
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
		return Coded(http.StatusBadRequest, "badPath", "invalid path")
	case errors.Is(err, ccfs.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return Coded(http.StatusNotFound, "notFound", "path not found")
	case errors.Is(err, ccfs.ErrProtected):
		return Coded(http.StatusForbidden, "protected", "a rule protects something in this folder")
	case errors.Is(err, ccfs.ErrInCloud):
		return Coded(http.StatusConflict, "inCloud", "the file lies in the cloud only")
	case errors.Is(err, ccfs.ErrReadOnly):
		return Coded(http.StatusForbidden, "readOnly", "path is read only")
	case errors.Is(err, ccfs.ErrForbidden), errors.Is(err, ccfs.ErrEscape), errors.Is(err, fs.ErrPermission):
		return Coded(http.StatusForbidden, "denied", "access denied")
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
