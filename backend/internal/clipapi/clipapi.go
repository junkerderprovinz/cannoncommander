// Package clipapi lets the desktop app exchange files with the other
// programs on the computer: through the system clipboard, and by dragging
// files in from Explorer or a file manager and out to them. Files come and
// go as paths on disk, and only those inside a source reach the panels.
package clipapi

import (
	_ "embed"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var (
	errDesktopOnly = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app shares files with other programs")
	errNotOnDisk   = v1.Coded(http.StatusBadRequest, "notOnDisk", "only files on disk go to other programs")
	errClipboard   = v1.Coded(http.StatusBadGateway, "clipboardBusy", "the clipboard could not be reached")
	errDrag        = v1.Coded(http.StatusBadGateway, "dragFailed", "the files could not be dragged")
)

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("clipapi", contract)
	bind(r, handlers{r.Deps()})
}

func bind(r *v1.Router, h handlers) {
	r.Handle("GET /clipboard", h.files)
	r.Handle("PUT /clipboard", h.put)
	r.Handle("POST /clipboard/moved", h.moved)
	r.Handle("POST /dragdrop/in", h.dropped)
	r.Handle("POST /dragdrop/out", h.dragOut)
}

type handlers struct{ deps v1.Deps }

type filesJSON struct {
	Files []string `json:"files"`
	Cut   bool     `json:"cut"`
	// Skipped counts the files left out: outside every source, hidden from
	// the person or gone.
	Skipped int `json:"skipped"`
}

// files is GET /clipboard.
func (h handlers) files(w http.ResponseWriter, r *http.Request) error {
	c := desktop.SystemClipboard()
	if c == nil {
		return errDesktopOnly
	}
	paths, cut, err := c.ClipboardFiles()
	if err != nil {
		logger.Errorf("reading the clipboard: %v", err)
		return errClipboard
	}
	files, _, skipped := h.inside(v1.UserFrom(r), paths)
	return v1.WriteJSON(w, r, http.StatusOK, filesJSON{Files: files, Cut: cut, Skipped: skipped})
}

// put is PUT /clipboard.
func (h handlers) put(w http.ResponseWriter, r *http.Request) error {
	c := desktop.SystemClipboard()
	if c == nil {
		return errDesktopOnly
	}
	var in struct {
		Paths []string `json:"paths"`
		Cut   bool     `json:"cut"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	reals, err := h.onDisk(v1.UserFrom(r), in.Paths, in.Cut)
	if err != nil {
		return err
	}
	if err := c.CopyFiles(reals, in.Cut); err != nil {
		logger.Errorf("putting %d files on the clipboard: %v", len(reals), err)
		return errClipboard
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// moved is POST /clipboard/moved: the page moved the files it found cut on
// the clipboard. The clipboard is emptied only while it still holds them, so
// nothing copied since then gets lost.
func (h handlers) moved(w http.ResponseWriter, r *http.Request) error {
	c := desktop.SystemClipboard()
	if c == nil {
		return errDesktopOnly
	}
	var in struct {
		Files []string `json:"files"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	paths, cut, err := c.ClipboardFiles()
	if err != nil {
		logger.Errorf("reading the clipboard: %v", err)
		return errClipboard
	}
	held := make([]string, 0, len(paths))
	for _, p := range paths {
		if api, ok := apiPath(h.deps.FS.Sources(), p); ok {
			held = append(held, api)
		}
	}
	slices.Sort(held)
	moved := slices.Sorted(slices.Values(in.Files))
	if cut && len(held) == len(paths) && slices.Equal(held, moved) {
		if err := c.ClearClipboard(); err != nil {
			logger.Errorf("clearing the clipboard: %v", err)
			return errClipboard
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// dropped is POST /dragdrop/in: files another program dropped on the window,
// as paths on disk, and the folder they are going to.
func (h handlers) dropped(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	var in struct {
		Paths []string `json:"paths"`
		To    string   `json:"to"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u := v1.UserFrom(r)
	dest, err := h.deps.FS.Check(u, in.To, ccfs.View)
	if err != nil {
		return err
	}
	files, reals, skipped := h.inside(u, in.Paths)
	same := len(reals) > 0 && !dest.Virtual()
	for _, p := range reals {
		same = same && sameVolume(p, dest.Real)
	}
	return v1.WriteJSON(w, r, http.StatusOK, struct {
		filesJSON
		SameVolume bool `json:"sameVolume"`
	}{filesJSON{Files: files, Skipped: skipped}, same})
}

// dragOut is POST /dragdrop/out. It answers once the files are dropped
// somewhere or the drag is given up.
func (h handlers) dragOut(w http.ResponseWriter, r *http.Request) error {
	d := desktop.SystemDrag()
	if d == nil {
		return errDesktopOnly
	}
	var in struct {
		Paths []string `json:"paths"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	reals, err := h.onDisk(v1.UserFrom(r), in.Paths, false)
	if err != nil {
		return err
	}
	if err := d.DragFiles(reals); err != nil {
		logger.Errorf("dragging %d files out: %v", len(reals), err)
		return errDrag
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// inside turns paths on disk into the paths of the sources that hold them,
// leaving out what the person may not see there and what is gone. reals are
// the paths on disk of the files it keeps.
func (h handlers) inside(u *users.User, paths []string) (files, reals []string, skipped int) {
	files = []string{}
	for _, p := range paths {
		api, ok := apiPath(h.deps.FS.Sources(), p)
		if !ok {
			skipped++
			continue
		}
		loc, err := h.deps.FS.Entry(u, api, ccfs.View)
		if err != nil || loc.Virtual() || loc.InArchive() {
			skipped++
			continue
		}
		if _, err := os.Lstat(loc.Real); err != nil {
			skipped++
			continue
		}
		files, reals = append(files, api), append(reals, loc.Real)
	}
	return files, reals, skipped
}

// onDisk finds the files at paths on disk for another program, which may
// read them anywhere and, for a cut, move them away. A link has to lead to
// a place the person may take from as well.
func (h handlers) onDisk(u *users.User, paths []string, cut bool) ([]string, error) {
	perm := ccfs.View | ccfs.Download
	reals := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, err := h.deps.FS.Landed(u, p, perm); err != nil {
			return nil, err
		}
		take := perm
		if cut {
			take |= ccfs.Delete
		}
		loc, err := h.deps.FS.Entry(u, p, take)
		if err != nil {
			return nil, err
		}
		if loc.Virtual() || loc.InArchive() {
			return nil, errNotOnDisk.With("path", p)
		}
		reals = append(reals, loc.Real)
	}
	return reals, nil
}

// apiPath is the path a file on disk has in the source that holds it.
func apiPath(sources []ccfs.Source, real string) (string, bool) {
	if !filepath.IsAbs(real) {
		return "", false
	}
	real = filepath.Clean(real)
	for _, s := range sources {
		rel, err := filepath.Rel(s.Root, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		api := path.Join(s.Mount, filepath.ToSlash(rel))
		// A name with the separator of archives would read as a path into one.
		if strings.Contains(api, ccfs.ArchiveSep) {
			return "", false
		}
		return api, true
	}
	return "", false
}
