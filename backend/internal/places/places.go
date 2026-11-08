// Package places is what the desktop app shows of the computer itself: the
// folders every user has, such as Downloads, and the drives, which come and
// go as sticks are plugged in and shares are opened.
package places

import (
	_ "embed"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

//go:embed openapi.yaml
var contract []byte

var (
	errDesktopOnly = v1.Coded(http.StatusNotFound, "desktopOnly", "only the desktop app knows the computer's places")
	errNoVolume    = v1.Coded(http.StatusNotFound, "noVolume", "the computer has no such drive")
	errNoEject     = v1.Coded(http.StatusConflict, "ejectFailed", "the drive could not be ejected")
	errNoMount     = v1.Coded(http.StatusConflict, "mountFailed", "the drive could not be mounted")
)

// folder is one of the folders the computer keeps for every user. kind is
// home, desktop, documents, downloads, pictures, music or videos.
type folder struct {
	kind string
	path string
}

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("places", contract)
	bind(r, handlers{fs: r.Deps().FS, known: knownFolders})
}

func bind(r *v1.Router, h handlers) {
	r.Handle("GET /places", h.list)
	r.Handle("POST /places/eject", h.eject)
	r.Handle("POST /places/mount", h.mount)
}

type handlers struct {
	fs *ccfs.Resolver
	// known lists the folders the computer keeps for the user whose home
	// folder is given.
	known func(home string) []folder
}

type folderJSON struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type volumeJSON struct {
	ID     string `json:"id"`
	Path   string `json:"path,omitempty"`
	Label  string `json:"label,omitempty"`
	Kind   string `json:"kind"`
	Remote string `json:"remote,omitempty"`
	Size   uint64 `json:"size,omitempty"`
	Eject  bool   `json:"eject,omitempty"`
}

type mountedJSON struct {
	Path string `json:"path"`
}

type listJSON struct {
	Folders []folderJSON `json:"folders"`
	Volumes []volumeJSON `json:"volumes"`
}

func volumeFor(v desktop.Volume) volumeJSON {
	out := volumeJSON{ID: v.ID, Label: v.Label, Kind: v.Kind, Remote: v.Remote, Size: v.Size, Eject: v.Eject}
	if v.Root != "" {
		out.Path = apiPath(v.Root)
	}
	return out
}

// apiPath is the API path of a folder on disk, without the slash that ends
// one at the top of a drive.
func apiPath(dir string) string { return path.Clean(ccfs.MountFor(dir)) }

// list is GET /places.
func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	if !desktop.Active() {
		return errDesktopOnly
	}
	u := v1.UserFrom(r)
	out := listJSON{Folders: []folderJSON{}, Volumes: []volumeJSON{}}
	for _, f := range h.known(desktop.Home()) {
		p := apiPath(f.path)
		if info, err := os.Stat(f.path); err != nil || !info.IsDir() {
			continue
		}
		if _, err := h.fs.Check(u, p, ccfs.View); err == nil {
			out.Folders = append(out.Folders, folderJSON{Kind: f.kind, Path: p})
		}
	}
	if vols := desktop.CurrentVolumes(); vols != nil {
		for _, v := range vols.Volumes() {
			out.Volumes = append(out.Volumes, volumeFor(v))
		}
	}
	// A share opened by its path is no drive of the computer's, but it is
	// one of the places the window has been.
	for _, s := range h.fs.Volumes() {
		if strings.HasPrefix(s.Mount, "/net/") {
			out.Volumes = append(out.Volumes, volumeJSON{ID: s.Mount, Path: s.Mount, Label: s.Name, Kind: s.Drive.Kind, Remote: s.Drive.Remote})
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// volume finds the volume a request names.
func volume(r *http.Request) (desktop.Volumes, desktop.Volume, error) {
	if !desktop.Active() {
		return nil, desktop.Volume{}, errDesktopOnly
	}
	var in struct {
		ID string `json:"id"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return nil, desktop.Volume{}, err
	}
	vols := desktop.CurrentVolumes()
	if vols == nil {
		return nil, desktop.Volume{}, errNoVolume
	}
	all := vols.Volumes()
	at := slices.IndexFunc(all, func(v desktop.Volume) bool { return v.ID == in.ID })
	if at < 0 {
		return nil, desktop.Volume{}, errNoVolume
	}
	return vols, all[at], nil
}

// eject is POST /places/eject.
func (h handlers) eject(w http.ResponseWriter, r *http.Request) error {
	vols, v, err := volume(r)
	if err != nil {
		return err
	}
	if !v.Eject {
		return errNoEject
	}
	if err := vols.Eject(v.ID); err != nil {
		logger.Warningf("ejecting %s: %v", v.ID, err)
		return errNoEject
	}
	logger.Infof("ejected %s", v.ID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// mount is POST /places/mount.
func (h handlers) mount(w http.ResponseWriter, r *http.Request) error {
	vols, v, err := volume(r)
	if err != nil {
		return err
	}
	root := v.Root
	if root == "" {
		if root, err = vols.Mount(v.ID); err != nil {
			logger.Warningf("mounting %s: %v", v.ID, err)
			return errNoMount
		}
		logger.Infof("mounted %s at %s", v.ID, root)
	}
	return v1.WriteJSON(w, r, http.StatusOK, mountedJSON{Path: apiPath(root)})
}
