// Package launch opens a file in the desktop app as a double click in the
// computer's own file manager does: a document in its program, a program by
// running it.
package launch

import (
	_ "embed"
	"net/http"
	"os"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var (
	errDesktopOnly = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app opens a file in its program")
	errNotAFile    = v1.Coded(http.StatusBadRequest, "notAFile", "path is not a regular file")
	errAskRun      = v1.Coded(http.StatusConflict, "askRun", "the file is a program, run it only when asked to")
	errNotStarted  = v1.Coded(http.StatusBadGateway, "notStarted", "the file did not open")
)

// system is how the computer opens files.
type system interface {
	// asks reports whether file starts something of its own, which the
	// computer's file manager only does once the person says so.
	asks(file string) bool
	// open hands file to the program the computer has for it.
	open(file string) error
	// run starts file as a program.
	run(file string) error
}

// Routes registers the contract and the handler.
func Routes(r *v1.Router) {
	r.Contract("launch", contract)
	bind(r, &handlers{deps: r.Deps(), sys: newSystem()})
}

func bind(r *v1.Router, h *handlers) {
	r.Handle("POST /launch", h.launch)
}

type handlers struct {
	deps v1.Deps
	sys  system
}

// launch is POST /launch.
func (h *handlers) launch(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	var in struct {
		Path string `json:"path"`
		Run  bool   `json:"run"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	loc, err := h.deps.FS.Landed(v1.UserFrom(r), in.Path, ccfs.View|ccfs.Download)
	if err != nil {
		return err
	}
	if loc.Virtual() || loc.InArchive() {
		return errNotAFile
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errNotAFile
	}
	start := h.sys.open
	if h.sys.asks(loc.Real) {
		if !in.Run {
			return errAskRun
		}
		start = h.sys.run
	}
	if err := start(loc.Real); err != nil {
		logger.Errorf("opening %s: %v", loc.Real, err)
		return errNotStarted
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
