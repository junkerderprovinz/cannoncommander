package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/meta"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// KindReindex is the job that reads a source into the index again.
const KindReindex = "reindex"

func init() {
	jobs.RegisterKind(KindReindex, jobs.Kind{Prepare: prepareReindex, Run: runReindex})
}

// progressEvery is how often a rebuild reports how far the scanner got.
var progressEvery = 500 * time.Millisecond

// sourceIndex is what the admin page and a rebuild ask of the index of
// one source.
type sourceIndex interface {
	RequestFullScan() error
	FullPasses() uint64
	ActiveScan() string
	GetNumDirs() uint64
	GetNumFiles() uint64
	GetStatus() indexing.IndexStatus
	GetLastIndexed() time.Time
	GetQuickScanTime() int
	GetFullScanTime() int
	NextScan() time.Time
	ScanEvery() time.Duration
	IndexedSize() uint64
}

// indexOf returns the index of a source, or nil when it has none.
var indexOf = func(name string) sourceIndex {
	if idx := indexing.Lookup(name); idx != nil {
		return idx
	}
	return nil
}

// ReindexParams name the source to read again, or none for all of them.
type ReindexParams struct {
	Source string `json:"source,omitempty"`
}

func prepareReindex(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	if u == nil || !u.Permissions.Admin {
		return nil, v1.Errorf(http.StatusForbidden, "only admins rebuild the index")
	}
	var p ReindexParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	var items []jobs.Item
	for _, s := range d.FS.Sources() {
		if p.Source != "" && s.Name != p.Source {
			continue
		}
		if indexOf(s.Name) == nil {
			if p.Source != "" {
				return nil, v1.Errorf(http.StatusBadRequest, "the source "+p.Source+" has no index")
			}
			continue
		}
		items = append(items, jobs.Item{Path: s.Mount, Name: s.Name})
	}
	if len(items) == 0 {
		if p.Source != "" {
			return nil, v1.Errorf(http.StatusBadRequest, "there is no source called "+p.Source)
		}
		return nil, v1.Errorf(http.StatusBadRequest, "no source has an index")
	}
	name := "Alle Quellen"
	if len(items) == 1 {
		name = items[0].Name
	}
	return &jobs.Plan{Params: p, Name: name, Items: items}, nil
}

// indexProgress is the index.progress event.
type indexProgress struct {
	Job      string `json:"job"`
	Source   string `json:"source"`
	Status   string `json:"status"`
	Dirs     uint64 `json:"dirs"`
	Files    uint64 `json:"files"`
	Scanning string `json:"scanning,omitempty"`
	Done     bool   `json:"done,omitempty"`
}

func runReindex(ctx context.Context, run *jobs.Run) error {
	d := run.Deps()
	for _, it := range run.Items() {
		if it.State != jobs.ItemPending {
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		ix := indexOf(it.Name)
		if ix == nil {
			run.Finish(it.Seq, jobs.ItemFailed, errors.New("this source has no index any more"))
			continue
		}
		err := rebuild(ctx, run, it, ix)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			err = forgetGone(ctx, d.FS, it.Path)
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}

// rebuild asks the scheduler for a full pass and reports the scanner's
// counts until the pass is through.
func rebuild(ctx context.Context, run *jobs.Run, it jobs.Item, ix sourceIndex) error {
	before := ix.FullPasses()
	if err := ix.RequestFullScan(); err != nil {
		return err
	}
	t := time.NewTicker(progressEvery)
	defer t.Stop()
	for {
		done := ix.FullPasses() > before
		publishProgress(run, it.Name, ix, done)
		if done {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func publishProgress(run *jobs.Run, source string, ix sourceIndex, done bool) {
	h := run.Deps().Hub
	if h == nil {
		return
	}
	ev := indexProgress{
		Job: run.ID(), Source: source, Status: string(ix.GetStatus()), Dirs: ix.GetNumDirs(), Files: ix.GetNumFiles(),
		Scanning: ix.ActiveScan(), Done: done,
	}
	if _, err := h.PublishJSON(EventIndexProgress, "", ev, run.Owner()); err != nil {
		logger.Errorf("search: %v", err)
	}
}

// forgetGone drops the cached metadata of files that are gone, which a
// rebuild is the time for.
func forgetGone(ctx context.Context, fs *ccfs.Resolver, mount string) error {
	_, err := meta.Default().Forget(ctx, mount, func(p string) bool {
		loc, err := fs.Resolve(p)
		if err != nil || loc.Virtual() {
			return false
		}
		_, err = os.Lstat(loc.Real)
		return err == nil
	})
	return err
}

// IndexState is the state of the index of one source on the admin page.
type IndexState struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	// Status is ready, indexing, unavailable or off.
	Status      string `json:"status"`
	Dirs        uint64 `json:"dirs"`
	Files       uint64 `json:"files"`
	Size        uint64 `json:"size"`
	LastIndexed string `json:"lastIndexed,omitempty"`
	QuickScan   int    `json:"quickScanSeconds"`
	FullScan    int    `json:"fullScanSeconds"`
	// NextScan and ScanEvery are the plan of the scheduler, which reads busy
	// folders more often than quiet ones.
	NextScan  string `json:"nextScan,omitempty"`
	ScanEvery int    `json:"scanEverySeconds,omitempty"`
	Scanning  string `json:"scanning,omitempty"`
	// Metadata counts the files below the source with cached metadata.
	Metadata int64 `json:"metadata"`
	// Job is the rebuild that is open for the source.
	Job string `json:"job,omitempty"`
}

type indexStates struct {
	Sources []IndexState `json:"sources"`
}

// IndexRoutes binds GET and POST /admin/index, whose contract is part of
// the system package.
func IndexRoutes(r *v1.Router) {
	h := handlers{deps: r.Deps()}
	r.Handle("GET /admin/index", h.indexState)
	r.Handle("POST /admin/index", h.indexRebuild)
}

func (h handlers) indexState(w http.ResponseWriter, r *http.Request) error {
	open := map[string]string{}
	if m := jobs.For(h.deps); m != nil {
		for _, j := range m.List("") {
			if j.Kind != KindReindex || !j.State.Open() {
				continue
			}
			full, _ := m.Get(j.ID, true)
			for _, it := range full.Items {
				open[it.Name] = j.ID
			}
		}
	}
	out := indexStates{Sources: []IndexState{}}
	for _, s := range h.deps.FS.Sources() {
		st := IndexState{Source: s.Name, Path: s.Mount, Status: "off", Job: open[s.Name]}
		if ix := indexOf(s.Name); ix != nil {
			st.Status = string(ix.GetStatus())
			st.Dirs, st.Files, st.Size = ix.GetNumDirs(), ix.GetNumFiles(), ix.IndexedSize()
			if t := ix.GetLastIndexed(); !t.IsZero() {
				st.LastIndexed = formatTime(t)
			}
			if t := ix.NextScan(); !t.IsZero() {
				st.NextScan = formatTime(t)
			}
			st.ScanEvery = int(ix.ScanEvery() / time.Second)
			st.QuickScan, st.FullScan = ix.GetQuickScanTime(), ix.GetFullScanTime()
			st.Scanning = ix.ActiveScan()
		}
		if n, err := meta.Default().Count(r.Context(), s.Mount); err == nil {
			st.Metadata = n
		}
		out.Sources = append(out.Sources, st)
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func (h handlers) indexRebuild(w http.ResponseWriter, r *http.Request) error {
	m := jobs.For(h.deps)
	if m == nil {
		return v1.Errorf(http.StatusServiceUnavailable, "the job queue is not running")
	}
	raw := json.RawMessage("{}")
	if r.ContentLength != 0 {
		var p ReindexParams
		if err := v1.DecodeJSON(r, &p); err != nil {
			return err
		}
		raw, _ = json.Marshal(p)
	}
	u := v1.UserFrom(r)
	plan, err := prepareReindex(r.Context(), h.deps, u, raw)
	if err != nil {
		return err
	}
	j, err := m.Submit(u.Username, KindReindex, plan, 0, false)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, j)
}
