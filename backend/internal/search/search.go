package search

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

// Limits of one search. The list shows 2000 hits at most, as the
// prototype does, and the search window can ask for more.
const (
	DefaultLimit = 2000
	MaxLimit     = 10000
	// metaRead is how long a field search reads metadata the cache does
	// not hold yet. The files it did not get to count as pending.
	metaRead = 10 * time.Second
)

// Hit is one search result: a listing row and the folder it lies in.
type Hit struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
	// SizeUnknown marks a folder the index has not measured yet.
	SizeUnknown bool   `json:"sizeUnknown,omitempty"`
	MTime       string `json:"mtime"`
	BTime       string `json:"btime,omitempty"`
	Mode        string `json:"mode"`
	Hidden      bool   `json:"hidden,omitempty"`
	At          string `json:"at"`
	// Line is where a content search found the text first, counted from 1.
	Line int `json:"line,omitempty"`
}

// Result is the answer of GET /search.
type Result struct {
	Path  string `json:"path"`
	Items []Hit  `json:"items"`
	// Total counts every hit, More tells that Items holds only the first.
	Total   int  `json:"total"`
	More    bool `json:"more,omitempty"`
	Scanned int  `json:"scanned"`
	// Pending counts files whose metadata could not be read in time, so a
	// field search may have missed them.
	Pending int      `json:"pending,omitempty"`
	Unknown []string `json:"unknown,omitempty"`
	// Indexed tells that the search read the index rather than the disk.
	Indexed bool `json:"indexed"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0)
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func hitOf(r *row) Hit {
	h := Hit{
		Name: r.Name, Dir: r.Dir, Size: r.Size, MTime: formatTime(r.MTime), At: r.At,
		Mode: fmt.Sprintf("%04o", r.mode()), Hidden: r.Hidden || ccfs.HiddenByName(r.Name),
	}
	if !r.Created.IsZero() {
		h.BTime = formatTime(r.Created)
	}
	if r.Dir {
		h.Size = 0
	}
	return h
}

// collector keeps the first hits in the order the window shows them,
// folders first and names as people sort them, without holding every hit
// of a search over a whole array.
type collector struct {
	limit int
	total int
	rows  []*row
	coll  *collate.Collator
}

func newCollector(limit int) *collector {
	return &collector{limit: limit, coll: collate.New(language.German, collate.Numeric)}
}

func (c *collector) add(r *row) {
	c.total++
	c.rows = append(c.rows, r)
	if len(c.rows) >= 2*c.limit {
		c.trim()
	}
}

func (c *collector) trim() {
	slices.SortStableFunc(c.rows, func(a, b *row) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return c.coll.CompareString(a.Name, b.Name)
	})
	if len(c.rows) > c.limit {
		c.rows = c.rows[:c.limit]
	}
}

func (h handlers) search(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	if q.Get("text") != "" {
		return v1.Errorf(http.StatusBadRequest, "a search for text inside files runs as a job: POST /jobs with the kind contentSearch")
	}
	p := Params{
		Path: q.Get("path"), Q: q.Get("q"), Type: q.Get("type"), Min: q.Get("min"), Max: q.Get("max"), When: q.Get("when"),
	}
	if hv := q.Get("hidden"); hv != "" {
		b, err := strconv.ParseBool(hv)
		if err != nil {
			return v1.Errorf(http.StatusBadRequest, "hidden is true or false")
		}
		p.Hidden = b
	}
	limit := DefaultLimit
	if lv := q.Get("limit"); lv != "" {
		n, err := strconv.Atoi(lv)
		if err != nil || n < 1 || n > MaxLimit {
			return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("limit is a number from 1 to %d", MaxLimit))
		}
		limit = n
	}
	u := v1.UserFrom(r)
	loc, err := checkParams(h.deps.FS, u, p)
	if err != nil {
		return err
	}
	f := h.finder(r.Context(), u, p)
	out := Result{Path: loc.Path, Items: []Hit{}, Unknown: f.q.Unknown}
	if len(f.q.Unknown) > 0 {
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	f.readUntil = time.Now().Add(metaRead)
	c := newCollector(limit)
	if err := f.run(r.Context(), loc, func(row *row) error { c.add(row); return nil }); err != nil {
		return err
	}
	c.trim()
	for _, row := range c.rows {
		out.Items = append(out.Items, hitOf(row))
	}
	h.folderSizes(out.Items, c.rows)
	out.Total, out.More, out.Scanned, out.Pending, out.Indexed = c.total, c.total > len(c.rows), f.scanned, f.pending, f.indexed
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// checkParams checks the criteria of a search and the folder it starts in.
func checkParams(fs *ccfs.Resolver, u *users.User, p Params) (ccfs.Location, error) {
	if p.Path == "" {
		return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, "path is required")
	}
	if !ValidType(p.Type) {
		return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, "type is one of all, dir, img, video, audio, doc and arch")
	}
	if !validWhen(p.When) {
		return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, "when is one of any, 1, 7, 30, 365 and old")
	}
	for name, v := range map[string]string{"min": p.Min, "max": p.Max} {
		if v != "" && math.IsNaN(Bytes(v)) {
			return ccfs.Location{}, v1.Errorf(http.StatusBadRequest, name+" is a size such as 300 KB or 2,5 GB")
		}
	}
	loc, err := fs.Check(u, p.Path, ccfs.View)
	if err != nil {
		return loc, err
	}
	if loc.InArchive() {
		return loc, v1.Errorf(http.StatusNotImplemented, "searching inside archives is not built yet")
	}
	if loc.Virtual() {
		return loc, nil
	}
	if err := fs.Follows(u, loc, ccfs.View); err != nil {
		return loc, err
	}
	if loc, err = fs.Followed(loc); err != nil {
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

func (h handlers) finder(ctx context.Context, u *users.User, p Params) *finder {
	f := newFinder(h.deps.FS, u, p, time.Now())
	if tags := f.q.Tags(); len(tags) > 0 {
		f.tags = taggedPaths(ctx, h.deps.DB, tags)
	}
	if h.deps.Index != nil {
		f.sizes = func(e fsindex.Entry) int64 {
			loc, err := h.deps.FS.Resolve(e.Path)
			if err != nil || loc.Source == nil {
				return 0
			}
			size, _ := h.deps.Index.FolderSize(loc.Source, loc.Index)
			return size
		}
	}
	return f
}

// folderSizes fills in the total size of the folders among the hits, as
// far as the index has measured them, and marks the others.
func (h handlers) folderSizes(items []Hit, rows []*row) {
	for i, r := range rows {
		items[i].SizeUnknown = r.Dir
	}
	if h.deps.Index == nil {
		return
	}
	type ref struct {
		i     int
		index string
	}
	bySource := map[*ccfs.Source][]ref{}
	sources := map[string]*ccfs.Source{}
	for i, r := range rows {
		if !r.Dir {
			continue
		}
		loc, err := h.deps.FS.Resolve(r.Path)
		if err != nil || loc.Source == nil {
			continue
		}
		src, ok := sources[loc.Source.Name]
		if !ok {
			src = loc.Source
			sources[src.Name] = src
		}
		bySource[src] = append(bySource[src], ref{i, loc.Index})
	}
	for src, refs := range bySource {
		dirs := make([]string, len(refs))
		for k, x := range refs {
			dirs[k] = x.index
		}
		got := h.deps.Index.Folders(src, dirs)
		for _, x := range refs {
			if f, ok := got[x.index]; ok && f.Size >= 0 {
				items[x.i].Size = f.Size
				items[x.i].SizeUnknown = false
			}
		}
	}
}
