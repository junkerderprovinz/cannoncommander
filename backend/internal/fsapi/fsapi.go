// Package fsapi serves the reading side of /fs: listings, the folder tree
// and, later, metadata, previews and content.
package fsapi

import (
	_ "embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers that exist.
func Routes(r *v1.Router) {
	r.Contract("fsapi", contract)
	h := handlers{fs: r.Deps().FS}
	r.Handle("GET /fs/list", h.list)
	r.Handle("GET /fs/tree", h.tree)
}

// Item is one row of a listing.
type Item struct {
	Name   string `json:"name"`
	Dir    bool   `json:"dir"`
	Size   int64  `json:"size"`
	MTime  string `json:"mtime"`
	Mode   string `json:"mode"`
	Owner  string `json:"owner,omitempty"`
	Group  string `json:"group,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Link   string `json:"link,omitempty"`
}

// Volume is the source a listed folder belongs to, with its fill level.
type Volume struct {
	Path  string `json:"path"`
	Label string `json:"label,omitempty"`
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

// Listing is the answer of GET /fs/list.
type Listing struct {
	Path   string  `json:"path"`
	Items  []Item  `json:"items"`
	Volume *Volume `json:"volume,omitempty"`
}

// TreeItem is one subfolder in the answer of GET /fs/tree. Label, Free and
// Total are set for the folders where a source is mounted.
type TreeItem struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	HasDirs bool   `json:"hasDirs"`
	Label   string `json:"label,omitempty"`
	Free    uint64 `json:"free,omitempty"`
	Total   uint64 `json:"total,omitempty"`
}

// Tree is the answer of GET /fs/tree.
type Tree struct {
	Path  string     `json:"path"`
	Items []TreeItem `json:"items"`
}

type handlers struct {
	fs *ccfs.Resolver
}

func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, err := h.folder(u, r.URL.Query().Get("path"))
	if err != nil {
		return err
	}
	out := Listing{Path: loc.Path, Items: []Item{}}
	if loc.Virtual() {
		for _, name := range h.fs.VirtualChildren(loc.Path) {
			if h.fs.Visible(u, loc, name) {
				out.Items = append(out.Items, h.virtualItem(path.Join(loc.Path, name), name))
			}
		}
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	dirents, err := os.ReadDir(loc.Real)
	if err != nil {
		return err
	}
	for _, d := range dirents {
		if !h.fs.Visible(u, loc, d.Name()) {
			continue
		}
		item, err := itemFor(filepath.Join(loc.Real, d.Name()), d.Name())
		if err != nil {
			// The entry vanished between reading the folder and looking at it.
			continue
		}
		out.Items = append(out.Items, item)
	}
	out.Volume = volumeOf(loc.Source)
	// Free space changes all the time, the ETag only follows the entries.
	return v1.WriteJSONTagged(w, r, http.StatusOK, out, Listing{Path: out.Path, Items: out.Items})
}

func (h handlers) tree(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, err := h.folder(u, r.URL.Query().Get("path"))
	if err != nil {
		return err
	}
	out := Tree{Path: loc.Path, Items: []TreeItem{}}
	var names []string
	if loc.Virtual() {
		names = h.fs.VirtualChildren(loc.Path)
	} else {
		dirents, err := os.ReadDir(loc.Real)
		if err != nil {
			return err
		}
		for _, d := range dirents {
			if isDir(loc.Real, d) {
				names = append(names, d.Name())
			}
		}
	}
	for _, name := range names {
		if !h.fs.Visible(u, loc, name) {
			continue
		}
		child, err := h.fs.Check(u, path.Join(loc.Path, name), ccfs.View)
		if err != nil {
			continue
		}
		item := TreeItem{Name: name, Path: child.Path, HasDirs: h.hasSubfolder(u, child)}
		if child.Source != nil && child.Index == "/" {
			item.Label = child.Source.Name
			if v := volumeOf(child.Source); v != nil {
				item.Free, item.Total = v.Free, v.Total
			}
		}
		out.Items = append(out.Items, item)
	}
	return v1.WriteJSONTagged(w, r, http.StatusOK, out, withoutUsage(out))
}

// folder checks that p is a folder the user may list.
func (h handlers) folder(u *users.User, p string) (ccfs.Location, error) {
	if p == "" {
		return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, "path is required")
	}
	loc, err := h.fs.Check(u, p, ccfs.View)
	if err != nil {
		return loc, err
	}
	if loc.InArchive() {
		return loc, v1.Errorf(http.StatusNotImplemented, "listing archives is not built yet")
	}
	if loc.Virtual() {
		return loc, nil
	}
	if err := ccfs.Contained(loc); err != nil {
		return loc, err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, err
	}
	if !info.IsDir() {
		return loc, v1.Errorf(http.StatusBadRequest, "path is not a folder")
	}
	return loc, nil
}

// hasSubfolder stops at the first visible subfolder, so the tree does not
// read every folder of the next level in full.
func (h handlers) hasSubfolder(u *users.User, dir ccfs.Location) bool {
	if dir.Virtual() {
		for _, name := range h.fs.VirtualChildren(dir.Path) {
			if h.fs.Visible(u, dir, name) {
				return true
			}
		}
		return false
	}
	if ccfs.Contained(dir) != nil {
		return false
	}
	f, err := os.Open(dir.Real)
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		batch, err := f.ReadDir(256)
		for _, d := range batch {
			if isDir(dir.Real, d) && h.fs.Visible(u, dir, d.Name()) {
				return true
			}
		}
		if err != nil {
			return false
		}
	}
}

func (h handlers) virtualItem(p, name string) Item {
	item := Item{Name: name, Dir: true, Mode: "0755", MTime: formatTime(time.Time{})}
	if loc, err := h.fs.Resolve(p); err == nil && !loc.Virtual() {
		if info, err := os.Stat(loc.Real); err == nil {
			item.MTime = formatTime(info.ModTime())
			item.Mode = modeString(info.Mode())
			item.Owner, item.Group = ownerOf(info)
		}
	}
	return item
}

func volumeOf(s *ccfs.Source) *Volume {
	if s == nil {
		return nil
	}
	free, total, ok := usage(s.Root)
	if !ok {
		return nil
	}
	return &Volume{Path: s.Mount, Label: s.Name, Free: free, Total: total}
}

func itemFor(real, name string) (Item, error) {
	info, err := os.Lstat(real)
	if err != nil {
		return Item{}, err
	}
	item := Item{Name: name, Hidden: strings.HasPrefix(name, ".")}
	if info.Mode()&fs.ModeSymlink != 0 {
		if target, err := os.Readlink(real); err == nil {
			item.Link = filepath.ToSlash(target)
		}
		// A link shows what it points to, a broken one stays a plain entry.
		if t, err := os.Stat(real); err == nil {
			info = t
		}
	}
	item.Dir = info.IsDir()
	if !item.Dir {
		item.Size = info.Size()
	}
	item.MTime = formatTime(info.ModTime())
	item.Mode = modeString(info.Mode())
	item.Owner, item.Group = ownerOf(info)
	return item, nil
}

func isDir(parent string, d fs.DirEntry) bool {
	if d.IsDir() {
		return true
	}
	if d.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(parent, d.Name()))
	return err == nil && info.IsDir()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0)
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func modeString(m fs.FileMode) string {
	return fmt.Sprintf("%04o", m.Perm())
}

// withoutUsage drops the fill levels, so the tree's ETag only follows the
// folders themselves.
func withoutUsage(t Tree) Tree {
	items := make([]TreeItem, len(t.Items))
	for i, it := range t.Items {
		it.Free, it.Total = 0, 0
		items[i] = it
	}
	return Tree{Path: t.Path, Items: items}
}
