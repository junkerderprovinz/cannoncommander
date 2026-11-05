package tools

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
)

// scanTools are the tools that walk a whole tree: disk usage and the
// duplicate search.
type scanTools struct {
	deps v1.Deps
}

// Largest files the disk usage window lists, as the prototype does.
const (
	defaultLargest = 200
	maxLargest     = 2000
)

// Usage is the answer of GET /tools/diskusage: one folder measured as a
// tree, by kind of file and by its largest files.
type Usage struct {
	Path string `json:"path"`
	UsageTotals
	Items   []UsageItem `json:"items"`
	Types   []UsageType `json:"types"`
	Largest []BigFile   `json:"largest"`
	Indexed bool        `json:"indexed"`
}

// UsageTotals measure a folder: the bytes of its files, how many files and
// folders lie anywhere below it and its newest change.
type UsageTotals struct {
	Size  int64  `json:"size"`
	Files int64  `json:"files"`
	Dirs  int64  `json:"dirs"`
	MTime string `json:"mtime"`
	mtime time.Time
}

func (t *UsageTotals) add(e fsindex.Entry) {
	if e.Dir {
		t.Dirs++
	} else {
		t.Files++
		t.Size += e.Size
	}
	if e.MTime.After(t.mtime) {
		t.mtime = e.MTime
	}
}

// UsageItem is one entry directly inside the folder.
type UsageItem struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	UsageTotals
}

// UsageType sums the files of one kind, with the extensions that take the
// most room.
type UsageType struct {
	Kind  string   `json:"kind"`
	Name  string   `json:"name"`
	Files int64    `json:"files"`
	Size  int64    `json:"size"`
	Exts  []string `json:"exts"`
}

// BigFile is one of the largest files. At is its folder.
type BigFile struct {
	Name  string `json:"name"`
	At    string `json:"at"`
	Size  int64  `json:"size"`
	MTime string `json:"mtime"`
}

var usageKindNames = map[string]string{
	"img": "Bild", "video": "Video", "audio": "Musik", "arch": "Archiv", "disk": "Abbild", "doc": "Dokument",
	"text": "Text", "code": "Quelltext", "file": "Sonstiges",
}

// usageKind folds PDF and office files into one kind, as the window
// groups them.
func usageKind(name string) string {
	switch k := search.KindOf(name, false); k {
	case "pdf", "office":
		return "doc"
	default:
		return k
	}
}

func stamp(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0)
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func (h scanTools) diskUsage(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	hidden := false
	if v := q.Get("hidden"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return v1.Errorf(http.StatusBadRequest, "hidden is true or false")
		}
		hidden = b
	}
	largest := defaultLargest
	if v := q.Get("largest"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > maxLargest {
			return v1.Errorf(http.StatusBadRequest, "largest is a number from 0 to "+strconv.Itoa(maxLargest))
		}
		largest = n
	}
	kind := q.Get("kind")
	u := v1.UserFrom(r)
	loc, err := scanRoot(h.deps.FS, u, q.Get("path"))
	if err != nil {
		return err
	}
	out, err := measureUsage(r.Context(), h.deps.FS, u, loc, hidden, kind, largest)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// scanRoot checks that p is a folder inside a source the user may see.
func scanRoot(fsys *ccfs.Resolver, u *users.User, p string) (ccfs.Location, error) {
	if p == "" {
		return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, "path is required")
	}
	loc, err := fsys.Landed(u, p, ccfs.View)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() {
		return loc, v1.Errorf(http.StatusBadRequest, "pick a folder inside a source")
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

// scanVisible is the common filter of the tools that walk a tree: it passes
// over CannonCommander's own files, hidden entries unless wanted and
// whatever the user may not see.
func scanVisible(fsys *ccfs.Resolver, u *users.User, e fsindex.Entry, hidden bool) bool {
	if search.Bookkeeping(e.Name) || (!hidden && (e.Hidden || strings.HasPrefix(e.Name, "."))) {
		return false
	}
	_, err := fsys.Check(u, e.Path, ccfs.View)
	return err == nil
}

func measureUsage(ctx context.Context, fsys *ccfs.Resolver, u *users.User, loc ccfs.Location, hidden bool, kind string, largest int) (Usage, error) {
	out := Usage{Path: loc.Path, Items: []UsageItem{}, Types: []UsageType{}, Largest: []BigFile{}}
	children := map[string]*UsageItem{}
	types := map[string]*UsageType{}
	extSize := map[string]map[string]int64{}
	var big []fsindex.Entry
	prefix := strings.TrimSuffix(loc.Path, "/") + "/"
	indexed, err := fsindex.Walk(ctx, loc, func(e fsindex.Entry) error {
		if !scanVisible(fsys, u, e, hidden) {
			return fs.SkipDir
		}
		out.UsageTotals.add(e)
		rel := strings.TrimPrefix(e.Path, prefix)
		name, below, inside := strings.Cut(rel, "/")
		c := children[name]
		if c == nil {
			c = &UsageItem{Name: name}
			children[name] = c
		}
		if !inside || below == "" {
			c.Dir = e.Dir
			if !e.Dir {
				c.UsageTotals.add(e)
			} else if e.MTime.After(c.mtime) {
				c.mtime = e.MTime
			}
		} else {
			c.UsageTotals.add(e)
		}
		if e.Dir {
			return nil
		}
		k := usageKind(e.Name)
		t := types[k]
		if t == nil {
			t = &UsageType{Kind: k, Name: usageKindNames[k]}
			types[k] = t
			extSize[k] = map[string]int64{}
		}
		t.Files++
		t.Size += e.Size
		if ext := strings.TrimPrefix(strings.ToLower(path.Ext(e.Name)), "."); ext != "" {
			extSize[k][ext] += e.Size
		}
		if largest > 0 && (kind == "" || kind == k) {
			big = append(big, e)
			if len(big) >= 4*largest {
				big = topFiles(big, largest)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	out.Indexed = indexed
	out.MTime = stamp(out.mtime)
	for _, c := range children {
		c.MTime = stamp(c.mtime)
		out.Items = append(out.Items, *c)
	}
	slices.SortFunc(out.Items, func(a, b UsageItem) int {
		if a.Size != b.Size {
			if a.Size > b.Size {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	for k, t := range types {
		exts := make([]string, 0, len(extSize[k]))
		for e := range extSize[k] {
			exts = append(exts, e)
		}
		slices.SortFunc(exts, func(a, b string) int {
			if d := extSize[k][b] - extSize[k][a]; d != 0 {
				if d > 0 {
					return 1
				}
				return -1
			}
			return strings.Compare(a, b)
		})
		t.Exts = exts[:min(5, len(exts))]
		out.Types = append(out.Types, *t)
	}
	slices.SortFunc(out.Types, func(a, b UsageType) int {
		if a.Size != b.Size {
			if a.Size > b.Size {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	for _, e := range topFiles(big, largest) {
		out.Largest = append(out.Largest, BigFile{Name: e.Name, At: path.Dir(e.Path), Size: e.Size, MTime: stamp(e.MTime)})
	}
	return out, nil
}

// topFiles keeps the n largest files, largest first.
func topFiles(files []fsindex.Entry, n int) []fsindex.Entry {
	slices.SortFunc(files, func(a, b fsindex.Entry) int {
		if a.Size != b.Size {
			if a.Size > b.Size {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	return files[:min(n, len(files))]
}
