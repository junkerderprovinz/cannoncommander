// Package openwith opens a file in the desktop app with one of the programs
// the computer offers for it, as the system's own file manager does.
package openwith

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"net/http"
	"os"
	"slices"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"golang.org/x/image/draw"
)

//go:embed openapi.yaml
var contract []byte

// iconSize is the edge of an app's icon in pixels, twice the glyph of a
// menu row, so it stays sharp on a screen with twice the pixels.
const iconSize = 32

var (
	errDesktopOnly = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app opens a file in another program")
	errNotAFile    = v1.Coded(http.StatusBadRequest, "notAFile", "path is not a regular file")
	errUnknownApp  = v1.Coded(http.StatusBadRequest, "unknownApp", "the computer offers no such program for this file")
	errNoDialog    = v1.Coded(http.StatusNotFound, "noAppDialog", "this computer has no dialog to pick a program")
	errNotStarted  = v1.Coded(http.StatusBadGateway, "appNotStarted", "the program did not start")

	errNoIcon = errors.New("no icon")
)

// app is a program the computer offers for a file.
type app struct {
	// id is how the system names the program: its path or the name of a
	// packaged app on Windows, the bundle on a Mac and the desktop entry on
	// Linux.
	id   string
	name string
	// icon is where the system keeps the program's icon, and the key it is
	// cached under.
	icon string
	def  bool
}

// system is what the computer offers.
type system interface {
	// apps lists the programs for file, the default one first.
	apps(file string) ([]app, error)
	// every lists the programs a person can pick from on a computer
	// without a dialog of its own for that.
	every() ([]app, error)
	// icon returns the icon a program keeps at where as a data URL.
	icon(where string) (string, error)
	open(file, id string) error
	// choose shows the system's dialog that picks a program for file and
	// opens it there. It returns once the dialog closes.
	choose(file string) error
	// dialog reports whether the system has such a dialog.
	dialog() bool
}

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("openwith", contract)
	bind(r, newHandlers(r.Deps(), newSystem()))
}

func bind(r *v1.Router, h *handlers) {
	r.Handle("GET /openwith", h.list)
	r.Handle("POST /openwith", h.open)
	r.Handle("POST /openwith/dialog", h.dialog)
}

type handlers struct {
	deps v1.Deps
	sys  system

	mu    sync.Mutex
	icons map[string]string
}

func newHandlers(deps v1.Deps, sys system) *handlers {
	return &handlers{deps: deps, sys: sys, icons: map[string]string{}}
}

type appJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Icon    string `json:"icon,omitempty"`
	Default bool   `json:"default,omitempty"`
}

type listJSON struct {
	Apps   []appJSON `json:"apps"`
	Dialog bool      `json:"dialog"`
}

// list is GET /openwith.
func (h *handlers) list(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	q := r.URL.Query()
	loc, err := h.file(v1.UserFrom(r), q.Get("path"))
	if err != nil {
		return err
	}
	apps, err := h.offered(loc.Real, q.Get("every") == "true")
	if err != nil {
		return err
	}
	out := make([]appJSON, 0, len(apps))
	for _, a := range apps {
		out = append(out, appJSON{ID: a.id, Name: a.name, Icon: h.icon(a.icon), Default: a.def})
	}
	return v1.WriteJSON(w, r, http.StatusOK, listJSON{Apps: out, Dialog: h.sys.dialog()})
}

// open is POST /openwith.
func (h *handlers) open(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	var in struct {
		Path  string `json:"path"`
		App   string `json:"app"`
		Every bool   `json:"every"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	loc, err := h.file(v1.UserFrom(r), in.Path)
	if err != nil {
		return err
	}
	apps, err := h.offered(loc.Real, in.Every)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(apps, func(a app) bool { return a.id == in.App })
	if at < 0 {
		return errUnknownApp
	}
	if err := h.sys.open(loc.Real, in.App); err != nil {
		logger.Errorf("opening %s with %s: %v", loc.Real, in.App, err)
		return errNotStarted.With("app", apps[at].name)
	}
	logger.Infof("opened %s with %s", loc.Real, apps[at].name)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// dialog is POST /openwith/dialog. The dialog stays open after the answer,
// as long as the person takes to pick.
func (h *handlers) dialog(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	if !h.sys.dialog() {
		return errNoDialog
	}
	var in struct {
		Path string `json:"path"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	loc, err := h.file(v1.UserFrom(r), in.Path)
	if err != nil {
		return err
	}
	go func() {
		if err := h.sys.choose(loc.Real); err != nil {
			logger.Errorf("picking a program for %s: %v", loc.Real, err)
		}
	}()
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// offered is what the computer offers for file, or every program there is.
func (h *handlers) offered(file string, every bool) ([]app, error) {
	if !every {
		return h.sys.apps(file)
	}
	if h.sys.dialog() {
		return nil, errNoDialog
	}
	return h.sys.every()
}

// file finds the file on disk. Its links are followed, so the rules judge
// the file the program really opens.
func (h *handlers) file(u *users.User, p string) (ccfs.Location, error) {
	loc, err := h.deps.FS.Landed(u, p, ccfs.View|ccfs.Download)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() {
		return loc, errNotAFile
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, err
	}
	if !info.Mode().IsRegular() {
		return loc, errNotAFile
	}
	return loc, nil
}

// icon returns the icon at where as a data URL, or "" when there is none,
// and keeps it, since a menu asks for the same few again and again.
func (h *handlers) icon(where string) string {
	if where == "" {
		return ""
	}
	h.mu.Lock()
	url, ok := h.icons[where]
	h.mu.Unlock()
	if ok {
		return url
	}
	url, err := h.sys.icon(where)
	if err != nil {
		logger.Debugf("no icon at %s: %v", where, err)
	}
	h.mu.Lock()
	h.icons[where] = url
	h.mu.Unlock()
	return url
}

// defaultFirst moves the default program to the top. Only the first one
// marked counts.
func defaultFirst(apps []app) []app {
	at := slices.IndexFunc(apps, func(a app) bool { return a.def })
	for i := range apps {
		apps[i].def = i == at
	}
	if at > 0 {
		def := apps[at]
		copy(apps[1:at+1], apps[:at])
		apps[0] = def
	}
	return apps
}

func dataURL(mediaType string, b []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// pngIcon scales img into the square of an icon in the menu.
func pngIcon(img image.Image) (string, error) {
	b := img.Bounds()
	if b.Empty() {
		return "", errors.New("empty image")
	}
	w, h := iconSize, iconSize
	if b.Dx() > b.Dy() {
		h = max(1, iconSize*b.Dy()/b.Dx())
	} else {
		w = max(1, iconSize*b.Dx()/b.Dy())
	}
	small := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	at := image.Rect((iconSize-w)/2, (iconSize-h)/2, (iconSize-w)/2+w, (iconSize-h)/2+h)
	draw.CatmullRom.Scale(small, at, img, b, draw.Over, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, small); err != nil {
		return "", err
	}
	return dataURL("image/png", out.Bytes()), nil
}
