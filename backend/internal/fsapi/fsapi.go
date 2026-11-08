// Package fsapi serves the reading side of /fs: listings, the folder tree,
// details of one path, file content, thumbnails, text and hex views.
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
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers that exist.
func Routes(r *v1.Router) {
	r.Contract("fsapi", contract)
	h := handlers{fs: r.Deps().FS, index: r.Deps().Index, thumbs: previewThumbs{}}
	r.Handle("GET /fs/list", h.list)
	r.Handle("GET /fs/tree", h.tree)
	r.Handle("GET /fs/stat", h.stat)
	r.Handle("GET /fs/raw", h.raw)
	r.Handle("GET /fs/thumb", h.thumb)
	r.Handle("GET /fs/text", h.text)
	r.Handle("GET /fs/hex", h.hex)
	r.Handle("GET /fs/file", h.fileGet)
	r.Handle("PUT /fs/file", h.filePut)
	r.Handle("POST /fs/archive-stream", h.archiveStream)
	meta.Routes(r, meta.Default())
}

// Item is one row of a listing.
type Item struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
	// SizeUnknown marks a folder the index has not measured yet, whose Size
	// is no total.
	SizeUnknown bool   `json:"sizeUnknown,omitempty"`
	MTime       string `json:"mtime"`
	Mode        string `json:"mode"`
	Owner       string `json:"owner,omitempty"`
	Group       string `json:"group,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	// System marks a protected file of the system, which stays hidden
	// until that is asked for as well.
	System bool `json:"system,omitempty"`
	// Attrs are the attributes of Windows as Explorer's column shows them.
	Attrs string `json:"attrs,omitempty"`
	// Cloud is the state of a file a cloud provider keeps: online, local
	// or pinned.
	Cloud string `json:"cloud,omitempty"`
	Link  string `json:"link,omitempty"`
	// Items counts the entries inside a folder, when known.
	Items   *int    `json:"items,omitempty"`
	Format  string  `json:"format,omitempty"`
	Picture string  `json:"picture,omitempty"`
	Pool    string  `json:"pool,omitempty"`
	Remote  string  `json:"remote,omitempty"`
	BTime   string  `json:"btime,omitempty"`
	Ino     uint64  `json:"ino,omitempty"`
	Nlink   uint64  `json:"nlink,omitempty"`
	UID     *uint32 `json:"uid,omitempty"`
	GID     *uint32 `json:"gid,omitempty"`
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
	Remote  string `json:"remote,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
	System  bool   `json:"system,omitempty"`
}

// Tree is the answer of GET /fs/tree.
type Tree struct {
	Path  string     `json:"path"`
	Items []TreeItem `json:"items"`
}

// Stat is the answer of GET /fs/stat. Files and Dirs count everything below
// a folder, as far as the index knows it.
type Stat struct {
	Path string `json:"path"`
	Item
	Files *int64 `json:"files,omitempty"`
	Dirs  *int64 `json:"dirs,omitempty"`
	// Devices are the disks and pools that hold a path of the user shares.
	Devices []string `json:"devices,omitempty"`
	Volume  *Volume  `json:"volume,omitempty"`
}

type handlers struct {
	fs     *ccfs.Resolver
	index  fsindex.Index
	thumbs thumbnailer
}

// bookkeeping reports the names CannonCommander keeps for itself: the trash folder of a share and
// the part file a copy writes before it takes the real name.
func bookkeeping(name string) bool {
	return name == trash.DirName || (strings.HasPrefix(name, ".") && strings.HasSuffix(name, fileops.PartSuffix))
}

func (h handlers) list(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, err := h.folder(u, r.URL.Query().Get("path"))
	if err != nil {
		return err
	}
	if loc.InArchive() {
		return h.archiveList(w, r, loc)
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
	seen := h.seenFrom(loc)
	listed := hiddenListed(loc.Real)
	for _, d := range dirents {
		if bookkeeping(d.Name()) || !h.fs.Visible(u, seen, d.Name()) {
			continue
		}
		item, err := itemFor(h.fs.RealIn(loc, d.Name()), d.Name())
		if err != nil {
			// The entry vanished between reading the folder and looking at it.
			continue
		}
		item.Hidden = item.Hidden || listed[d.Name()]
		out.Items = append(out.Items, item)
	}
	refs := make([]folderRef, 0, len(out.Items))
	for i := range out.Items {
		refs = append(refs, refIn(loc, &out.Items[i]))
	}
	h.describe(loc.Source, refs)
	unraidDetails(loc, out.Items)
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
	if loc.InArchive() {
		return h.archiveTree(w, r, loc)
	}
	out := Tree{Path: loc.Path, Items: []TreeItem{}}
	var names []string
	hidden, system := map[string]bool{}, map[string]bool{}
	if loc.Virtual() {
		names = h.fs.VirtualChildren(loc.Path)
	} else {
		dirents, err := os.ReadDir(loc.Real)
		if err != nil {
			return err
		}
		listed := hiddenListed(loc.Real)
		for _, d := range dirents {
			if isDir(loc.Real, d) {
				names = append(names, d.Name())
				hidden[d.Name()] = ccfs.HiddenEntry(d) || listed[d.Name()]
				system[d.Name()] = ccfs.ProtectedEntry(d)
			}
		}
	}
	seen := h.seenFrom(loc)
	for _, name := range names {
		if bookkeeping(name) || !h.fs.Visible(u, seen, name) {
			continue
		}
		child, err := h.fs.Check(u, path.Join(loc.Path, name), ccfs.View)
		if err != nil {
			continue
		}
		item := TreeItem{Name: name, Path: child.Path, Remote: remoteAt(child.Path), Hidden: hidden[name], System: system[name]}
		// Reading a disk to find a folder on it would wake it, and a disk
		// without any folder is rare enough to show the toggle anyway.
		if child.Source != nil && child.Source.Device && child.Index == "/" {
			item.HasDirs = true
		} else {
			item.HasDirs = h.hasSubfolder(u, child)
		}
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

func (h handlers) stat(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	loc, err := h.fs.Check(u, r.URL.Query().Get("path"), ccfs.View)
	if err != nil {
		return err
	}
	if loc.InArchive() {
		return h.archiveStat(w, r, loc)
	}
	out := Stat{Path: loc.Path}
	if loc.Virtual() {
		out.Item = h.virtualItem(loc.Path, path.Base(loc.Path))
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	if err := h.fs.Follows(u, loc, ccfs.View); err != nil {
		return err
	}
	if out.Item, err = itemFor(loc.Real, path.Base(loc.Path)); err != nil {
		return err
	}
	ownerOf(loc.Real, &out.Item)
	if out.Dir && out.Link == "" {
		h.describe(loc.Source, []folderRef{{index: loc.Index, real: loc.Real, path: loc.Path, item: &out.Item}})
		if h.index != nil {
			if files, dirs, ok := h.index.Totals(loc.Source, loc.Index); ok {
				out.Files, out.Dirs = &files, &dirs
			}
		}
	}
	out.Devices = devicesOf(loc, &out.Item)
	out.Volume = volumeOf(loc.Source)
	return v1.WriteJSONTagged(w, r, http.StatusOK, out, Stat{Path: out.Path, Item: out.Item, Files: out.Files, Dirs: out.Dirs, Devices: out.Devices})
}

// seenFrom is the place the names in dir are judged by. Inside a linked folder
// the rules go by where the link leads, since the link's own path names nothing.
func (h handlers) seenFrom(dir ccfs.Location) ccfs.Location {
	if dir.Virtual() {
		return dir
	}
	if target, err := h.fs.Followed(dir); err == nil {
		return target
	}
	return dir
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
		return loc, h.archiveFile(u, loc)
	}
	if loc.Virtual() {
		return loc, nil
	}
	if err := h.fs.Follows(u, loc, ccfs.View); err != nil {
		return loc, err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, err
	}
	if !info.IsDir() {
		return loc, v1.Coded(http.StatusBadRequest, "notAFolder", "path is not a folder")
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
	if h.fs.Follows(u, dir, ccfs.View) != nil {
		return false
	}
	seen := h.seenFrom(dir)
	f, err := os.Open(dir.Real)
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		batch, err := f.ReadDir(256)
		for _, d := range batch {
			if isDir(dir.Real, d) && h.fs.Visible(u, seen, d.Name()) {
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
		if st, err := statFile(loc.Real); err == nil {
			fill(&item, st)
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
	st, err := lstatFile(real)
	if err != nil {
		return Item{}, err
	}
	item := Item{Name: name}
	item.Hidden, item.System = hiddenOf(name, st.attrs)
	item.Attrs = attrLetters(st.attrs)
	if st.attrs&(attrReparse|attrOffline|attrPinned|attrUnpinned|attrRecallOnDataAccess) != 0 {
		item.Cloud = cloudState(st.attrs, !ccfs.CloudOnly(st.attrs) && cloudTagged(real, st.attrs))
	}
	if st.mode&fs.ModeSymlink != 0 {
		if target, err := os.Readlink(real); err == nil {
			item.Link = filepath.ToSlash(target)
		}
		// A link shows what it points to, a broken one stays a plain entry.
		if t, err := statFile(real); err == nil {
			st = t
		}
	}
	fill(&item, st)
	if !item.Dir {
		item.Format = FormatOf(name)
	}
	return item, nil
}

func fill(item *Item, st fileStat) {
	item.Dir = st.isDir()
	item.Size = 0
	if !item.Dir {
		item.Size = st.size
	}
	item.MTime = formatTime(st.mtime)
	item.Mode = modeString(st.mode)
	item.Owner, item.Group = ownerNames(st)
	if !st.btime.IsZero() {
		item.BTime = formatTime(st.btime)
	}
	item.Ino, item.Nlink = st.ino, st.nlink
	if st.ids {
		uid, gid := st.uid, st.gid
		item.UID, item.GID = &uid, &gid
	}
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

// modeString writes the permission bits as chmod takes them, with setuid,
// setgid and sticky in front.
func modeString(m fs.FileMode) string {
	bits := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		bits |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		bits |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		bits |= 0o1000
	}
	return fmt.Sprintf("%04o", bits)
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
