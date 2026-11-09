//go:build desktop

// Package openwith opens a file in the desktop app with one of the programs
// the computer offers for it, as the system's own file manager does.
package openwith

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

//go:embed openapi.yaml
var contract []byte

// iconSize is the edge of an app's icon in pixels, twice the glyph of a
// menu row, so it stays sharp on a screen with twice the pixels.
const iconSize = 32

var (
	errNotAFile    = v1.Coded(http.StatusBadRequest, "notAFile", "path is not a regular file")
	errUnknownApp  = v1.Coded(http.StatusBadRequest, "unknownApp", "the computer offers no such program for this file")
	errNoDialog    = v1.Coded(http.StatusNotFound, "noAppDialog", "this computer has no dialog to pick a program")
	errNotStarted  = v1.Coded(http.StatusBadGateway, "appNotStarted", "the program did not start")
	errNotAProgram = v1.Coded(http.StatusBadRequest, "notAProgram", "this is no program the computer can start")
	errBadType     = v1.Coded(http.StatusBadRequest, "badFileType", "a file type is a few letters and digits, such as txt")

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
	// open hands file to the program id, which the system offers for files
	// that end in ext.
	open(file, id, ext string) error
	// choose shows the system's dialog that picks a program for file and
	// opens it there. It returns once the dialog closes.
	choose(file string) error
	// dialog reports whether the system has such a dialog.
	dialog() bool
	// pick shows the system's dialog for choosing a program and returns the
	// one chosen, with an empty id when the person closes it. title heads
	// the dialog and programs names its filter.
	pick(title, programs string) (app, error)
	// runs reports whether program is something run can start.
	runs(program string, info os.FileInfo) bool
	// run starts program with file, without a shell between them.
	run(program, file string) error
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
	r.Handle("POST /openwith/pick", h.pick)
	r.Handle("POST /openwith/program", h.program)
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

// list is GET /openwith. With type instead of a path it lists the programs
// for files of that type, which the editor setting picks from.
func (h *handlers) list(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	file, done, err := h.typed(v1.UserFrom(r), q.Get("path"), q.Get("type"))
	if err != nil {
		return err
	}
	defer done()
	apps, err := h.offered(file, q.Get("every") == "true")
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
	var in struct {
		Path  string `json:"path"`
		App   string `json:"app"`
		Every bool   `json:"every"`
		Type  string `json:"type"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	loc, err := h.file(v1.UserFrom(r), in.Path)
	if err != nil {
		return err
	}
	via, done, err := h.typed(v1.UserFrom(r), in.Path, in.Type)
	if err != nil {
		return err
	}
	defer done()
	apps, err := h.offered(via, in.Every)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(apps, func(a app) bool { return a.id == in.App })
	if at < 0 {
		return errUnknownApp
	}
	if err := h.sys.open(loc.Real, in.App, filepath.Ext(via)); err != nil {
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

// pick is POST /openwith/pick. It answers once the person has chosen, with
// no content when they closed the dialog.
func (h *handlers) pick(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Title    string `json:"title"`
		Programs string `json:"programs"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	a, err := h.sys.pick(in.Title, in.Programs)
	if err != nil {
		return err
	}
	if a.id == "" {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	return v1.WriteJSON(w, r, http.StatusOK, appJSON{ID: a.id, Name: a.name, Icon: h.icon(a.icon)})
}

// program is POST /openwith/program: a file opens in a program the person
// picked as a file of its own. The rules judge the program as they judge the
// file.
func (h *handlers) program(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Path    string `json:"path"`
		Program string `json:"program"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u := v1.UserFrom(r)
	loc, err := h.file(u, in.Path)
	if err != nil {
		return err
	}
	prog, err := h.deps.FS.Landed(u, in.Program, ccfs.View|ccfs.Download)
	if err != nil {
		return err
	}
	if prog.Virtual() || prog.InArchive() {
		return errNotAProgram
	}
	info, err := os.Stat(prog.Real)
	if err != nil {
		return err
	}
	if !h.sys.runs(prog.Real, info) {
		return errNotAProgram
	}
	name := strings.TrimSuffix(filepath.Base(prog.Real), filepath.Ext(prog.Real))
	if err := h.sys.run(prog.Real, loc.Real); err != nil {
		logger.Errorf("opening %s with %s: %v", loc.Real, prog.Real, err)
		return errNotStarted.With("app", name)
	}
	logger.Infof("opened %s with %s", loc.Real, prog.Real)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// fileType is an extension without its dot.
var fileType = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)

// typed is the file whose programs count: the one at p, or with t a stand-in
// of that type, since the systems tell programs by a file. done removes the
// stand-in.
func (h *handlers) typed(u *users.User, p, t string) (file string, done func(), err error) {
	done = func() {}
	if t == "" {
		loc, err := h.file(u, p)
		return loc.Real, done, err
	}
	if !fileType.MatchString(t) {
		return "", done, errBadType
	}
	dir, err := os.MkdirTemp("", "cc-type-")
	if err != nil {
		return "", done, err
	}
	done = func() { _ = os.RemoveAll(dir) }
	file = filepath.Join(dir, "file."+t)
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		done()
		return "", func() {}, err
	}
	return file, done, nil
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
	b, err := xdg.ScaleIcon(img, iconSize)
	if err != nil {
		return "", err
	}
	return dataURL("image/png", b), nil
}
