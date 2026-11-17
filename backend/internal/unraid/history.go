package unraid

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// keepDays is how far back the history of a pool or disk reaches, a week
// more than the longest view of twelve weeks.
const keepDays = 13 * 7

// saveEvery spaces out the writes of the history file. Between them the
// day's sample follows the disk in memory.
const saveEvery = time.Hour

// Sample is how full a pool or disk was on one day: the day counted from
// 1970 and the bytes in use at the last look that day. It is written as a
// pair of numbers, which keeps the file small.
type Sample struct {
	Day  int64
	Used int64
}

func (s Sample) MarshalJSON() ([]byte, error) { return json.Marshal([2]int64{s.Day, s.Used}) }

func (s *Sample) UnmarshalJSON(b []byte) error {
	var p [2]int64
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	s.Day, s.Used = p[0], p[1]
	return nil
}

// Track is what the history keeps of one pool or disk. Times are Unix
// seconds.
type Track struct {
	Days []Sample `json:"days,omitempty"`
	// Temp is the last temperature read while the disk was awake; Unraid
	// shows none for a sleeping one.
	Temp        *int       `json:"temp,omitempty"`
	TempAt      int64      `json:"tempAt,omitempty"`
	AsleepSince int64      `json:"asleepSince,omitempty"`
	Zfs         *PoolFacts `json:"zfs,omitempty"`
}

// History holds the tracks of every pool and disk and keeps them in a file
// when it has one. It is safe for concurrent use.
type History struct {
	path string

	mu     sync.Mutex
	tracks map[string]*Track
	// awake is the spin state of each disk at the last look, which tells
	// when one fell asleep.
	awake   map[string]bool
	dirty   bool
	savedAt time.Time
}

// NewHistory reads the history from path. With an empty path, or a file
// that cannot be read, it starts empty.
func NewHistory(path string) *History {
	h := &History{path: path, tracks: map[string]*Track{}, awake: map[string]bool{}}
	if path == "" {
		return h
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	var file struct {
		Tracks map[string]*Track `json:"tracks"`
	}
	if json.Unmarshal(raw, &file) == nil && file.Tracks != nil {
		h.tracks = file.Tracks
	}
	return h
}

func (h *History) track(name string) *Track {
	tr := h.tracks[name]
	if tr == nil {
		tr = &Track{}
		h.tracks[name] = tr
	}
	return tr
}

// Observe takes in what Unraid reports right now. It reports whether a
// disk fell asleep or woke up, which is worth saving at once.
func (h *History) Observe(st *State, now time.Time) (spun bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	today := now.Unix() / 86400
	for _, d := range st.Disks {
		if d.Kind != "data" && d.Kind != "pool" && d.Kind != "parity" {
			continue
		}
		tr := h.track(d.Name)
		was, seen := h.awake[d.Name]
		h.awake[d.Name] = !d.Spundown
		switch {
		case !d.Spundown:
			tr.AsleepSince = 0
			if d.Temp != nil {
				t := *d.Temp
				tr.Temp, tr.TempAt = &t, now.Unix()
			}
		case seen && was:
			tr.AsleepSince = now.Unix()
		}
		spun = spun || seen && was == d.Spundown
		if used, ok := usedOf(d); ok {
			tr.add(Sample{Day: today, Used: used})
		}
		h.dirty = true
	}
	return spun
}

// usedOf is the bytes in use on a pool or data disk, from emhttp's own
// count. Parity and the second device of a pool have no file system.
func usedOf(d Disk) (int64, bool) {
	if d.Kind == "parity" || d.Pool != "" || !d.Mounted || d.FsSize <= 0 {
		return 0, false
	}
	if d.Used > 0 {
		return d.Used, true
	}
	return d.FsSize - d.Free, true
}

func (tr *Track) add(s Sample) {
	if n := len(tr.Days); n > 0 && tr.Days[n-1].Day == s.Day {
		tr.Days[n-1] = s
	} else {
		tr.Days = append(tr.Days, s)
	}
	cut := 0
	for cut < len(tr.Days) && tr.Days[cut].Day < s.Day-keepDays {
		cut++
	}
	tr.Days = tr.Days[cut:]
}

// Samples returns a copy of the samples of one pool or disk, oldest first.
func (h *History) Samples(name string) []Sample {
	h.mu.Lock()
	defer h.mu.Unlock()
	if tr := h.tracks[name]; tr != nil {
		return slices.Clone(tr.Days)
	}
	return nil
}

// Track returns a copy of what the history keeps of one pool or disk.
func (h *History) Track(name string) Track {
	h.mu.Lock()
	defer h.mu.Unlock()
	tr := h.tracks[name]
	if tr == nil {
		return Track{}
	}
	out := *tr
	out.Days = slices.Clone(tr.Days)
	return out
}

func (h *History) keepFacts(name string, f PoolFacts) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.track(name).Zfs = &f
	h.dirty = true
}

// Save writes the history file, through a temporary file so a crash
// leaves the old one.
func (h *History) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.path == "" {
		return nil
	}
	raw, err := json.Marshal(map[string]any{"tracks": h.tracks})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, h.path); err != nil {
		return err
	}
	h.dirty = false
	return nil
}

// saveDue reports whether the file is behind and the last write is long
// enough ago, or something happened that should not wait.
func (h *History) saveDue(now time.Time, urgent bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.dirty || h.path == "" {
		return false
	}
	if urgent || now.Sub(h.savedAt) >= saveEvery {
		h.savedAt = now
		return true
	}
	return false
}

// FullIn estimates in how many weeks the disk is full, from a straight line
// through the samples of the last weeks. ok is false while there is less
// than a week to go by, when the line is flat or falls, or when it would
// take more than two years, which so short a stretch cannot say.
func FullIn(samples []Sample, size int64, weeks int) (float64, bool) {
	if len(samples) == 0 || size <= 0 {
		return 0, false
	}
	last := samples[len(samples)-1]
	var pts []Sample
	for _, s := range samples {
		if s.Day > last.Day-int64(weeks)*7 {
			pts = append(pts, s)
		}
	}
	if len(pts) < 3 || last.Day-pts[0].Day < 7 {
		return 0, false
	}
	var mx, my float64
	for _, p := range pts {
		mx += float64(p.Day)
		my += float64(p.Used)
	}
	mx /= float64(len(pts))
	my /= float64(len(pts))
	var sxy, sxx float64
	for _, p := range pts {
		dx := float64(p.Day) - mx
		sxy += dx * (float64(p.Used) - my)
		sxx += dx * dx
	}
	perWeek := sxy / sxx * 7
	free := float64(size - last.Used)
	// Less than a thousandth of the disk a week counts as flat.
	if perWeek <= float64(size)/1000 || free <= 0 {
		return 0, false
	}
	w := free / perWeek
	if w > 104 {
		return 0, false
	}
	return w, true
}

// Observe looks at the server once and saves the history when it is due.
func (s *Service) Observe(now time.Time) {
	st, err := s.State()
	if err != nil || !st.Available {
		return
	}
	spun := s.history.Observe(st, now)
	if s.history.saveDue(now, spun) {
		if err := s.history.Save(); err != nil {
			logger.Errorf("unraid: drive history: %v", err)
		}
	}
}

// Record keeps the history of every pool and disk until ctx ends. It reads
// the same emhttp files as Watch, never a disk.
func (s *Service) Record(ctx context.Context) {
	t := time.NewTicker(s.cfg.Poll)
	defer t.Stop()
	for {
		s.Observe(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
