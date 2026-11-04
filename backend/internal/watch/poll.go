package watch

import (
	"errors"
	"io/fs"
	"os"
	"time"
)

// stamp is what the poller compares between two reads of a folder.
type stamp struct {
	size  int64
	mtime int64
	ino   uint64
	dir   bool
}

func (w *Watcher) pollLoop() {
	defer w.done.Done()
	tick := time.NewTicker(min(time.Second, w.opts.Poll/2))
	defer tick.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-tick.C:
			w.pollDue()
		}
	}
}

func (w *Watcher) pollDue() {
	now := time.Now()
	w.mu.Lock()
	var due []*folder
	for _, f := range w.folders {
		if f.poll && !now.Before(f.nextPoll) {
			due = append(due, f)
		}
	}
	w.mu.Unlock()
	for _, f := range due {
		start := time.Now()
		snap, err := snapshot(f.real)
		took := time.Since(start)
		w.mu.Lock()
		if w.folders[f.path] == f {
			// A folder that takes long to read is read less often, so a
			// huge folder on a slow mount does not keep the disks busy.
			f.nextPoll = time.Now().Add(max(w.opts.Poll, 20*took))
			w.compare(f, snap, err)
		}
		w.mu.Unlock()
	}
}

// compare turns the difference between two reads into a batch. The first
// read only sets the baseline. The caller holds mu.
func (w *Watcher) compare(f *folder, now map[string]stamp, err error) {
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && f.snap != nil {
			f.snap = nil
			w.note(f, func(b *batch) { b.gone = true })
		}
		return
	}
	old := f.snap
	f.snap = now
	if old == nil {
		return
	}
	var gone []string
	came := map[inoKey]string{}
	var cameNames []string
	for name := range old {
		if _, ok := now[name]; !ok {
			gone = append(gone, name)
		}
	}
	for name, s := range now {
		o, ok := old[name]
		switch {
		case !ok:
			cameNames = append(cameNames, name)
			if s.ino != 0 {
				came[inoKey{s.ino, s.dir}] = name
			}
		case o != s:
			w.note(f, func(b *batch) { b.add(name, s.dir) })
		}
	}
	renamed := map[string]bool{}
	for _, from := range gone {
		s := old[from]
		to, ok := came[inoKey{s.ino, s.dir}]
		if s.ino == 0 || !ok {
			w.note(f, func(b *batch) { b.remove(from, s.dir) })
			continue
		}
		renamed[to] = true
		// The inode is enough to show the rename to the client. Movers only
		// hear of inotify's paired events, because /mnt/user reports its own
		// inodes, which Unraid's mover can change under a file.
		w.note(f, func(b *batch) { b.rename(from, to, s.dir) })
	}
	for _, name := range cameNames {
		if !renamed[name] {
			w.note(f, func(b *batch) { b.add(name, now[name].dir) })
		}
	}
}

// inoKey pairs an entry that went away with one that came, which is how a
// rename looks to the poller.
type inoKey struct {
	ino uint64
	dir bool
}

func snapshot(dir string) (map[string]stamp, error) {
	dirents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]stamp, len(dirents))
	for _, d := range dirents {
		info, err := d.Info()
		if err != nil {
			continue
		}
		out[d.Name()] = stamp{size: info.Size(), mtime: info.ModTime().UnixNano(), ino: inoOf(info), dir: info.IsDir()}
	}
	return out, nil
}
