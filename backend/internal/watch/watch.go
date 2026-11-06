// Package watch reports changes inside the folders clients have open. It
// follows the hub's topics, so a folder is watched exactly while some
// client looks at it.
//
// inotify sees what happens through the path it watches. On /mnt/user that
// misses writes that go straight to a disk, and on network and rclone
// mounts it sees nothing, so such folders are compared with a fresh read
// every few seconds as well.
package watch

import (
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// EventChanged is published under hub.FolderTopic(folder) when something
// inside the folder appears, changes or goes away.
const EventChanged = "fs.changed"

// Change is the data of an fs.changed event. Names are entry names inside
// Path.
type Change struct {
	Path string `json:"path"`
	// Paths are the folders whose listing is stale: Path itself and the
	// subfolders that went away.
	Paths   []string `json:"paths"`
	Names   []string `json:"names,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Renamed []Rename `json:"renamed,omitempty"`
	// Reload says that too much happened to tell, read the folder again.
	Reload bool `json:"reload,omitempty"`
	// Gone says that the folder itself was deleted or moved away.
	Gone bool `json:"gone,omitempty"`
}

// Rename is an entry that got a new name inside the same folder.
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Mover hears about entries that something outside the app renamed or
// moved between watched folders, so data kept by path, such as labels, can
// follow them. Only renames inotify pairs by their cookie count, so a file
// that one disk loses and another gains, as when Unraid's mover runs, is
// never taken for one. Paths are absolute API paths.
type Mover interface {
	Moved(from, to string)
}

// Options tunes a watcher. Zero values pick the defaults.
type Options struct {
	// Delay bundles the changes of a folder into one event at most this
	// often.
	Delay time.Duration
	// Poll is how often folders that inotify cannot see are read again.
	Poll time.Duration
	// PollOnly skips inotify, which tests use to exercise the poller.
	PollOnly bool
}

// Watcher is safe for concurrent use.
type Watcher struct {
	opts Options

	mu      sync.Mutex
	hub     *hub.Hub
	fs      *ccfs.Resolver
	index   fsindex.Index
	wanted  map[string]bool
	folders map[string]*folder
	movers  []Mover
	notify  *notifier
	started bool
	closed  bool
	stop    chan struct{}
	done    sync.WaitGroup
}

type folder struct {
	path  string
	real  string
	src   *ccfs.Source
	index string

	watch int
	poll  bool
	// snap is the last poll's view, nil until the first poll.
	snap     map[string]stamp
	nextPoll time.Time

	batch *batch
	timer *time.Timer
}

// batch collects what happened in a folder until it is published.
type batch struct {
	names   map[string]bool
	removed map[string]bool
	dirs    map[string]bool
	renamed []Rename
	moves   [][2]string
	reload  bool
	gone    bool
}

// New returns a watcher that does nothing until Start.
func New(opts Options) *Watcher {
	if opts.Delay <= 0 {
		opts.Delay = 250 * time.Millisecond
	}
	if opts.Poll <= 0 {
		opts.Poll = 7 * time.Second
	}
	return &Watcher{opts: opts, wanted: map[string]bool{}, folders: map[string]*folder{}, stop: make(chan struct{})}
}

// Topic is the hub's OnTopic callback.
func (w *Watcher) Topic(topic string, active bool) {
	p, ok := strings.CutPrefix(topic, hub.FolderTopic(""))
	if !ok {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if active {
		w.wanted[p] = true
		if w.started {
			w.open(p)
		}
		return
	}
	delete(w.wanted, p)
	w.close(p)
}

// Start begins watching the folders subscribed so far and those to come.
// index may be nil, otherwise each changed folder is read into it again.
// movers hear about renames from outside.
func (w *Watcher) Start(h *hub.Hub, fs *ccfs.Resolver, index fsindex.Index, movers ...Mover) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.closed {
		return
	}
	w.hub, w.fs, w.index, w.movers, w.started = h, fs, index, movers, true
	if !w.opts.PollOnly {
		n, err := newNotifier(w)
		if err != nil {
			logger.Warningf("watch: inotify is not available, folders are polled: %v", err)
		} else {
			w.notify = n
		}
	}
	for p := range w.wanted {
		w.open(p)
	}
	w.done.Add(1)
	go w.pollLoop()
}

// Close stops all watches.
func (w *Watcher) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	for p := range w.folders {
		w.close(p)
	}
	n := w.notify
	w.notify = nil
	close(w.stop)
	w.mu.Unlock()
	if n != nil {
		n.close()
	}
	w.done.Wait()
}

// Watching returns the folders being watched, for tests and diagnostics.
func (w *Watcher) Watching() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.folders))
	for p := range w.folders {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// open starts watching p. Virtual folders change only with the
// configuration, so they are left alone.
func (w *Watcher) open(p string) {
	if _, ok := w.folders[p]; ok {
		return
	}
	loc, err := w.fs.Resolve(p)
	if err != nil || loc.Virtual() || loc.InArchive() || ccfs.Contained(loc) != nil {
		return
	}
	if info, err := os.Stat(loc.Real); err != nil || !info.IsDir() {
		return
	}
	f := &folder{path: loc.Path, real: loc.Real, src: loc.Source, index: loc.Index, watch: -1}
	if w.notify != nil {
		wd, err := w.notify.add(f.real, f.path)
		if err != nil {
			logger.Debugf("watch: inotify on %s: %v", f.real, err)
		} else {
			f.watch = wd
		}
	}
	f.poll = f.watch < 0 || needsPoll(f.real)
	f.nextPoll = time.Now()
	w.folders[p] = f
}

func (w *Watcher) close(p string) {
	f, ok := w.folders[p]
	if !ok {
		return
	}
	delete(w.folders, p)
	if f.timer != nil {
		f.timer.Stop()
	}
	if f.watch >= 0 && w.notify != nil {
		w.notify.remove(f.watch, f.path)
	}
}

// note records a change and arms the timer that publishes the batch.
// The caller holds mu.
func (w *Watcher) note(f *folder, change func(b *batch)) {
	if f.batch == nil {
		f.batch = &batch{names: map[string]bool{}, removed: map[string]bool{}, dirs: map[string]bool{}}
	}
	change(f.batch)
	if f.timer == nil {
		f.timer = time.AfterFunc(w.opts.Delay, func() { w.flush(f) })
	}
}

func (w *Watcher) flush(f *folder) {
	w.mu.Lock()
	b := f.batch
	f.batch, f.timer = nil, nil
	current := w.folders[f.path] == f
	h, fs, index, movers := w.hub, w.fs, w.index, w.movers
	w.mu.Unlock()
	if b == nil || !current {
		return
	}
	c := b.change(f.path)
	// A client reloads as soon as it hears of the change, so labels have to be on the new path by then.
	for _, m := range b.moves {
		for _, mv := range movers {
			mv.Moved(m[0], m[1])
		}
	}
	if _, err := h.PublishJSON(EventChanged, hub.FolderTopic(f.path), c); err != nil {
		logger.Errorf("watch: publish %s: %v", f.path, err)
	}
	if index == nil {
		return
	}
	// Rows of a moved folder are moved before the refresh, which would
	// otherwise read the folder at its new place all over again.
	for _, m := range b.moves {
		from, err1 := fs.Resolve(m[0])
		to, err2 := fs.Resolve(m[1])
		if err1 == nil && err2 == nil && from.Source != nil && to.Source != nil && to.Source.Name == from.Source.Name {
			index.Moved(from.Source, from.Index, to.Index)
		}
	}
	if b.gone {
		index.Removed(f.src, f.index)
		return
	}
	index.Refresh(f.src, f.index)
}

func (b *batch) change(dir string) Change {
	c := Change{Path: dir, Paths: []string{dir}, Reload: b.reload, Gone: b.gone}
	for n := range b.names {
		c.Names = append(c.Names, n)
	}
	for n := range b.removed {
		c.Removed = append(c.Removed, n)
		if b.dirs[n] {
			c.Paths = append(c.Paths, path.Join(dir, n))
		}
	}
	for _, r := range b.renamed {
		if b.dirs[r.From] {
			c.Paths = append(c.Paths, path.Join(dir, r.From))
		}
	}
	slices.Sort(c.Names)
	slices.Sort(c.Removed)
	slices.Sort(c.Paths[1:])
	c.Renamed = b.renamed
	return c
}

func (b *batch) add(name string, dir bool) {
	delete(b.removed, name)
	b.names[name] = true
	if dir {
		b.dirs[name] = true
	}
}

func (b *batch) remove(name string, dir bool) {
	delete(b.names, name)
	b.removed[name] = true
	if dir {
		b.dirs[name] = true
	}
}

func (b *batch) rename(from, to string, dir bool) {
	if b.names[from] {
		// Created and renamed within one batch: to the client it is new.
		delete(b.names, from)
		b.add(to, dir)
		return
	}
	delete(b.removed, from)
	delete(b.removed, to)
	b.renamed = append(b.renamed, Rename{From: from, To: to})
	if dir {
		b.dirs[from], b.dirs[to] = true, true
	}
}
