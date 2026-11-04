package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

type recorder struct {
	mu        sync.Mutex
	moves     [][2]string
	refreshed []string
}

func (r *recorder) Moved(from, to string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.moves = append(r.moves, [2]string{from, to})
}

func (r *recorder) Folders(*ccfs.Source, []string) map[string]fsindex.Folder { return nil }

func (r *recorder) Totals(*ccfs.Source, string) (int64, int64, bool) { return 0, 0, false }

func (r *recorder) Refresh(_ *ccfs.Source, dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshed = append(r.refreshed, dir)
}

func (r *recorder) seen() ([][2]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.moves), slices.Clone(r.refreshed)
}

type rig struct {
	root   string
	hub    *hub.Hub
	w      *Watcher
	rec    *recorder
	client *hub.Client
}

// modes runs a test against the poller everywhere and against inotify
// where the platform has it.
func modes(t *testing.T, run func(t *testing.T, opts Options)) {
	t.Run("poll", func(t *testing.T) { run(t, Options{PollOnly: true, Poll: 40 * time.Millisecond, Delay: 30 * time.Millisecond}) })
	if runtime.GOOS == "linux" {
		t.Run("inotify", func(t *testing.T) { run(t, Options{Delay: 30 * time.Millisecond}) })
	}
}

func newRig(t *testing.T, opts Options, folders ...string) *rig {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"downloads/unfertig", "filme"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{root: root, rec: &recorder{}}
	r.w = New(opts)
	r.hub = hub.New(hub.Options{OnTopic: r.w.Topic})
	r.w.Follow(r.rec)
	r.w.Start(r.hub, fs, r.rec)
	t.Cleanup(r.w.Close)
	r.client, _, _ = r.hub.Connect("tab", "jdp", 0)
	topics := make([]string, len(folders))
	for i, f := range folders {
		topics[i] = hub.FolderTopic(f)
	}
	r.hub.SetTopics("tab", "jdp", topics)
	// The poller needs one read as its baseline before it can see a change.
	if opts.PollOnly {
		time.Sleep(3 * opts.Poll)
	}
	return r
}

func (r *rig) file(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// next waits for the next fs.changed event and gathers what follows it
// within a short while, since a poll and inotify may split one change.
func (r *rig) next(t *testing.T) Change {
	t.Helper()
	select {
	case e := <-r.client.Events:
		var c Change
		if err := json.Unmarshal(e.Data, &c); err != nil {
			t.Fatal(err)
		}
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("no fs.changed event")
	}
	return Change{}
}

func (r *rig) quiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case e := <-r.client.Events:
		t.Fatalf("unexpected event %s %s", e.Topic, e.Data)
	case <-time.After(d):
	}
}

func TestNewFilesShowUp(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		r := newRig(t, opts, "/mnt/user/downloads")
		r.file(t, "downloads/film.mkv.part")
		c := r.next(t)
		if c.Path != "/mnt/user/downloads" || !slices.Contains(c.Names, "film.mkv.part") || c.Paths[0] != "/mnt/user/downloads" {
			t.Fatalf("change = %+v", c)
		}
		waitFor(t, func() bool { _, refreshed := r.rec.seen(); return slices.Contains(refreshed, "/downloads") })
	})
}

func TestChangesAreBundled(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		opts.Delay = 200 * time.Millisecond
		r := newRig(t, opts, "/mnt/user/downloads")
		for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
			r.file(t, "downloads/"+n+".txt")
		}
		got := map[string]bool{}
		events := 0
		for len(got) < 8 && events < 3 {
			for _, n := range r.next(t).Names {
				got[n] = true
			}
			events++
		}
		if len(got) != 8 || events > 2 {
			t.Fatalf("%d names in %d events", len(got), events)
		}
	})
}

func TestRenamesAreReportedAndFollowed(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		r := newRig(t, opts, "/mnt/user/downloads")
		from := r.file(t, "downloads/film.mkv.part")
		r.next(t)
		if err := os.Rename(from, filepath.Join(r.root, "downloads", "film.mkv")); err != nil {
			t.Fatal(err)
		}
		c := r.next(t)
		if runtime.GOOS == "windows" {
			// Without inodes the poller sees a file go and another come.
			if !slices.Equal(c.Removed, []string{"film.mkv.part"}) || !slices.Equal(c.Names, []string{"film.mkv"}) {
				t.Fatalf("change = %+v", c)
			}
			return
		}
		if len(c.Renamed) != 1 || c.Renamed[0] != (Rename{From: "film.mkv.part", To: "film.mkv"}) || len(c.Removed) != 0 {
			t.Fatalf("change = %+v", c)
		}
		moves, _ := r.rec.seen()
		if opts.PollOnly {
			if len(moves) != 0 {
				t.Fatalf("the poller told movers %v", moves)
			}
			return
		}
		if !slices.Contains(moves, [2]string{"/mnt/user/downloads/film.mkv.part", "/mnt/user/downloads/film.mkv"}) {
			t.Fatalf("moves = %v", moves)
		}
	})
}

func TestRemovedFoldersMarkTheirListingStale(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		r := newRig(t, opts, "/mnt/user/downloads")
		if err := os.Remove(filepath.Join(r.root, "downloads", "unfertig")); err != nil {
			t.Fatal(err)
		}
		c := r.next(t)
		if !slices.Equal(c.Removed, []string{"unfertig"}) || !slices.Equal(c.Paths, []string{"/mnt/user/downloads", "/mnt/user/downloads/unfertig"}) {
			t.Fatalf("change = %+v", c)
		}
	})
}

func TestAWatchedFolderThatGoesAwayIsReported(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		r := newRig(t, opts, "/mnt/user/downloads/unfertig")
		if err := os.Remove(filepath.Join(r.root, "downloads", "unfertig")); err != nil {
			t.Fatal(err)
		}
		if c := r.next(t); !c.Gone || c.Path != "/mnt/user/downloads/unfertig" {
			t.Fatalf("change = %+v", c)
		}
	})
}

func TestMovesBetweenWatchedFolders(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pairing a move across folders needs inotify")
	}
	r := newRig(t, Options{Delay: 30 * time.Millisecond}, "/mnt/user/downloads", "/mnt/user/filme")
	from := r.file(t, "downloads/Sintel.mkv")
	r.next(t)
	if err := os.Rename(from, filepath.Join(r.root, "filme", "Sintel.mkv")); err != nil {
		t.Fatal(err)
	}
	seen := map[string]Change{}
	for len(seen) < 2 {
		c := r.next(t)
		seen[c.Path] = c
	}
	if !slices.Equal(seen["/mnt/user/downloads"].Removed, []string{"Sintel.mkv"}) || !slices.Equal(seen["/mnt/user/filme"].Names, []string{"Sintel.mkv"}) {
		t.Fatalf("changes = %+v", seen)
	}
	moves, _ := r.rec.seen()
	if !slices.Contains(moves, [2]string{"/mnt/user/downloads/Sintel.mkv", "/mnt/user/filme/Sintel.mkv"}) {
		t.Fatalf("moves = %v", moves)
	}
}

func TestFoldersAreWatchedOnlyWhileSubscribed(t *testing.T) {
	modes(t, func(t *testing.T, opts Options) {
		r := newRig(t, opts, "/mnt/user/downloads", "/mnt", "/mnt/user/nichts")
		if got := r.w.Watching(); !slices.Equal(got, []string{"/mnt/user/downloads"}) {
			t.Fatalf("watching %v", got)
		}
		r.hub.SetTopics("tab", "jdp", nil)
		if got := r.w.Watching(); len(got) != 0 {
			t.Fatalf("still watching %v", got)
		}
		r.file(t, "downloads/spaeter.txt")
		r.quiet(t, 300*time.Millisecond)
	})
}

func TestTopicsBeforeStartAreWatchedOnStart(t *testing.T) {
	root := t.TempDir()
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	w := New(Options{PollOnly: true})
	h := hub.New(hub.Options{OnTopic: w.Topic})
	h.Subscribe("tab", "jdp", hub.FolderTopic("/mnt/user"), "labels")
	w.Start(h, fs, nil)
	defer w.Close()
	if got := w.Watching(); !slices.Equal(got, []string{"/mnt/user"}) {
		t.Fatalf("watching %v", got)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
