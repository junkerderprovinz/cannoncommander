//go:build desktop

// Package shellapi gives the desktop app's window what the computer's own
// file manager offers for files: its context menu, its properties dialog,
// starting a program as administrator, and its icons.
package shellapi

import (
	_ "embed"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
)

//go:embed openapi.yaml
var contract []byte

var (
	errNoMenu     = v1.Coded(http.StatusNotFound, "noSystemMenu", "this computer draws no menu of its own")
	errNoDialog   = v1.Coded(http.StatusNotFound, "noPropertiesDialog", "this computer has no properties dialog to show")
	errNotProgram = v1.Coded(http.StatusBadRequest, "notAProgram", "this file is no program the computer starts as administrator")
	errOneFolder  = v1.Coded(http.StatusBadRequest, "notOneFolder", "the items are not all in one folder")
	errUnknown    = v1.Coded(http.StatusBadRequest, "unknownAction", "no such action for these files")
	errNotInFront = v1.Coded(http.StatusConflict, "windowNotInFront", "the window is not in front")
	errFailed     = v1.Coded(http.StatusBadGateway, "systemFailed", "the computer could not do that")
)

// programs are the types Windows may offer to start as administrator. Only
// those it really offers that for are reported.
var programs = []string{".exe", ".com", ".bat", ".cmd", ".msi", ".msc", ".cpl", ".ps1", ".vbs", ".wsf", ".scr", ".lnk"}

// system is the computer as the routes use it, so the tests can stand in
// for it.
type system struct {
	nativeMenu    func() bool
	showMenu      func([]string, nativeshell.MenuOptions) ([]nativeshell.Entry, error)
	actions       func([]string) ([]nativeshell.Action, error)
	runAction     func([]string, string) error
	hasProperties func() bool
	properties    func([]string) error
	canRunAsAdmin func(string) bool
	runAsAdmin    func(string) error
	icon          func(string, int) ([]byte, string, error)
	typeIcon      func(string, bool, int) ([]byte, string, error)
}

var native = system{
	nativeMenu:    nativeshell.NativeMenu,
	showMenu:      nativeshell.ShowMenu,
	actions:       nativeshell.Actions,
	runAction:     nativeshell.RunAction,
	hasProperties: nativeshell.HasProperties,
	properties:    nativeshell.Properties,
	canRunAsAdmin: nativeshell.CanRunAsAdmin,
	runAsAdmin:    nativeshell.RunAsAdmin,
	icon:          nativeshell.Icon,
	typeIcon:      nativeshell.TypeIcon,
}

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("shell", contract)
	bind(r, &handlers{deps: r.Deps(), sys: native, seen: entries{r.Deps().DB}})
}

func bind(r *v1.Router, h *handlers) {
	r.Handle("GET /shell", h.offers)
	r.Handle("POST /shell/menu", h.menu)
	r.Handle("GET /shell/actions", h.listActions)
	r.Handle("POST /shell/actions", h.runAction)
	r.Handle("POST /shell/properties", h.properties)
	r.Handle("POST /shell/runas", h.runAs)
	r.Handle("GET /shell/icon", h.icon)
	r.Handle("GET /shell/entries", h.entries)
	r.Handle("PUT /shell/entries/hidden", h.hide)
}

type handlers struct {
	deps v1.Deps
	sys  system
	seen entries
}

type offersJSON struct {
	// Menu is "native" where the system draws its own menu, "actions"
	// where the app lists what other programs add.
	Menu       string   `json:"menu"`
	Properties bool     `json:"properties"`
	RunAs      []string `json:"runAs"`
}

// offers is GET /shell.
func (h *handlers) offers(w http.ResponseWriter, r *http.Request) error {
	out := offersJSON{Menu: "actions", Properties: h.sys.hasProperties(), RunAs: []string{}}
	if h.sys.nativeMenu() {
		out.Menu = "native"
	}
	for _, ext := range programs {
		if h.sys.canRunAsAdmin("program" + ext) {
			out.RunAs = append(out.RunAs, ext)
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// menu is POST /shell/menu. It answers once the menu has closed and the
// picked row has started.
func (h *handlers) menu(w http.ResponseWriter, r *http.Request) error {
	if !h.sys.nativeMenu() {
		return errNoMenu
	}
	var in struct {
		Paths      []string `json:"paths"`
		X          int      `json:"x"`
		Y          int      `json:"y"`
		Extended   bool     `json:"extended"`
		Background bool     `json:"background"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	files, err := h.items(v1.UserFrom(r), in.Paths)
	if err != nil {
		return err
	}
	if in.Background {
		if info, err := os.Stat(files[0]); err != nil || !info.IsDir() || len(files) > 1 {
			return v1.Errorf(http.StatusBadRequest, "the menu of a folder's empty space is for one folder")
		}
	}
	hidden, err := h.seen.hidden(r.Context())
	if err != nil {
		return err
	}
	seen, err := h.sys.showMenu(files, nativeshell.MenuOptions{X: in.X, Y: in.Y, Extended: in.Extended, Background: in.Background, Hidden: func(k string) bool { return hidden[k] }})
	if rerr := h.seen.remember(r.Context(), seen); rerr != nil {
		logger.Errorf("keeping the rows of the system menu: %v", rerr)
	}
	if err != nil {
		return failure("showing the system menu", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type actionJSON struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	Group string `json:"group,omitempty"`
}

// listActions is GET /shell/actions.
func (h *handlers) listActions(w http.ResponseWriter, r *http.Request) error {
	files, err := h.items(v1.UserFrom(r), r.URL.Query()["path"])
	if err != nil {
		return err
	}
	found, err := h.sys.actions(files)
	if err != nil {
		return failure("listing the actions of other programs", err)
	}
	seen := make([]nativeshell.Entry, 0, len(found))
	for _, a := range found {
		e := a.Entry
		if a.Group != "" {
			e.Label = a.Group + " › " + e.Label
		}
		seen = append(seen, e)
	}
	if err := h.seen.remember(r.Context(), seen); err != nil {
		logger.Errorf("keeping the actions of other programs: %v", err)
	}
	hidden, err := h.seen.hidden(r.Context())
	if err != nil {
		return err
	}
	out := []actionJSON{}
	for _, a := range found {
		if !hidden[a.Key] {
			out = append(out, actionJSON{Key: a.Key, Label: a.Label, Icon: a.Icon, Group: a.Group})
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"actions": out})
}

// runAction is POST /shell/actions.
func (h *handlers) runAction(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Paths []string `json:"paths"`
		Key   string   `json:"key"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	files, err := h.items(v1.UserFrom(r), in.Paths)
	if err != nil {
		return err
	}
	if err := h.sys.runAction(files, in.Key); err != nil {
		if errors.Is(err, nativeshell.ErrUnknownAction) {
			return errUnknown
		}
		return failure("running "+in.Key, err)
	}
	logger.Infof("ran the action %s on %d items", in.Key, len(files))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// properties is POST /shell/properties.
func (h *handlers) properties(w http.ResponseWriter, r *http.Request) error {
	if !h.sys.hasProperties() {
		return errNoDialog
	}
	var in struct {
		Paths []string `json:"paths"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	files, err := h.items(v1.UserFrom(r), in.Paths)
	if err != nil {
		return err
	}
	if err := h.sys.properties(files); err != nil {
		return failure("opening the system's properties", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// runAs is POST /shell/runas.
func (h *handlers) runAs(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Path string `json:"path"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	files, err := h.items(v1.UserFrom(r), []string{in.Path})
	if err != nil {
		return err
	}
	if info, err := os.Stat(files[0]); err != nil || !info.Mode().IsRegular() || !h.sys.canRunAsAdmin(files[0]) {
		return errNotProgram
	}
	if err := h.sys.runAsAdmin(files[0]); err != nil {
		return failure("starting "+files[0]+" as administrator", err)
	}
	logger.Infof("started %s as administrator", files[0])
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// iconSizes are the edges an icon comes in, those of the system's lists.
var iconSizes = map[string]int{"16": 16, "32": 32, "48": 48, "256": 256}

// icon is GET /shell/icon: the icon of one item, or of a type by a name.
func (h *handlers) icon(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	px, ok := iconSizes[q.Get("size")]
	if !ok {
		return v1.Errorf(http.StatusBadRequest, "size is 16, 32, 48 or 256")
	}
	var (
		data      []byte
		mediaType string
		err       error
	)
	if name := q.Get("name"); name != "" {
		data, mediaType, err = h.sys.typeIcon(filepath.Base(name), q.Get("dir") == "true", px)
		// A type keeps its icon until a program installs another one.
		w.Header().Set("Cache-Control", "private, max-age=86400")
	} else {
		files, ierr := h.items(v1.UserFrom(r), []string{q.Get("path")})
		if ierr != nil {
			return ierr
		}
		data, mediaType, err = h.sys.icon(files[0], px)
		// A client that puts the item's mtime into v may keep the icon for
		// good, because a changed file gets a new URL.
		if q.Get("v") != "" {
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "private, no-cache")
		}
	}
	if errors.Is(err, nativeshell.ErrNoPicture) || errors.Is(err, nativeshell.ErrUnsupported) {
		w.Header().Del("Cache-Control")
		return v1.Errorf(http.StatusNotFound, "the computer has no icon for this")
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An icon theme's SVG is a file from the computer, never a page.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	_, err = w.Write(data)
	return err
}

// entries is GET /shell/entries.
func (h *handlers) entries(w http.ResponseWriter, r *http.Request) error {
	list, err := h.seen.list(r.Context())
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"entries": list})
}

// hide is PUT /shell/entries/hidden.
func (h *handlers) hide(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Keys []string `json:"keys"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if err := h.seen.hide(r.Context(), in.Keys); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// items finds the selected items on disk, as they are and not where their
// links lead, since the menu of a link is the link's own.
func (h *handlers) items(u *users.User, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, v1.Errorf(http.StatusBadRequest, "paths are required")
	}
	files := make([]string, 0, len(paths))
	for _, p := range paths {
		loc, err := h.deps.FS.Check(u, p, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		if loc.Virtual() || loc.InArchive() {
			return nil, v1.Errorf(http.StatusBadRequest, p+" is not an item on this computer")
		}
		if _, err := os.Lstat(loc.Real); err != nil {
			return nil, err
		}
		if len(files) > 0 && filepath.Dir(loc.Real) != filepath.Dir(files[0]) {
			return nil, errOneFolder
		}
		files = append(files, loc.Real)
	}
	return files, nil
}

func failure(what string, err error) error {
	if errors.Is(err, nativeshell.ErrUnsupported) {
		return errNoMenu
	}
	if errors.Is(err, nativeshell.ErrNotInFront) {
		return errNotInFront
	}
	logger.Errorf("%s: %v", what, err)
	return errFailed
}
