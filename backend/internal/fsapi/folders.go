package fsapi

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

// peekBudget caps the time one answer spends reading subfolders the index
// does not know, so a folder with thousands of them still answers quickly.
// The folders after that go without a count.
const peekBudget = 150 * time.Millisecond

// folderRef points at a row that describe fills in when it is a folder.
type folderRef struct {
	index string
	real  string
	path  string
	item  *Item
}

func refIn(dir ccfs.Location, item *Item) folderRef {
	return folderRef{
		index: path.Join(dir.Index, item.Name),
		real:  filepath.Join(dir.Real, item.Name),
		path:  path.Join(dir.Path, item.Name),
		item:  item,
	}
}

// describe fills in the entry count, total size, folder picture and remote
// of the folder rows. The index answers for the folders it knows, the
// others are read directly within peekBudget. A folder whose size the
// index has not measured is marked, since its stat size of 0 would read as
// empty.
func (h handlers) describe(src *ccfs.Source, refs []folderRef) {
	var dirs []string
	for _, r := range refs {
		if r.item.Dir && r.item.Link == "" {
			dirs = append(dirs, r.index)
		}
	}
	var known map[string]fsindex.Folder
	if h.index != nil && len(dirs) > 0 {
		known = h.index.Folders(src, dirs)
	}
	deadline := time.Now().Add(peekBudget)
	for _, r := range refs {
		if !r.item.Dir {
			continue
		}
		r.item.Remote = remoteAt(r.path)
		// Following a link to count its target would read outside the
		// folder the user is looking at.
		if r.item.Link != "" {
			continue
		}
		f, ok := known[r.index]
		r.item.SizeUnknown = !ok || f.Size < 0
		if ok {
			n := f.Items
			r.item.Items = &n
			if f.Size >= 0 {
				r.item.Size = f.Size
			}
			r.item.Picture = pickPicture(f.Pictures)
			continue
		}
		if time.Now().After(deadline) {
			continue
		}
		if n, pics, ok := peek(r.real); ok {
			r.item.Items = &n
			r.item.Picture = pickPicture(pics)
		}
	}
}

func peek(dir string) (n int, pictures []string, ok bool) {
	f, err := os.Open(dir)
	if err != nil {
		return 0, nil, false
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return 0, nil, false
	}
	for _, name := range names {
		if pickPicture([]string{name}) != "" {
			pictures = append(pictures, name)
		}
	}
	return len(names), pictures, true
}

// pickPicture returns the most preferred folder picture among names.
func pickPicture(names []string) string {
	for _, want := range fsindex.FolderPictures {
		for _, n := range names {
			if strings.EqualFold(n, want) {
				return n
			}
		}
	}
	return ""
}
