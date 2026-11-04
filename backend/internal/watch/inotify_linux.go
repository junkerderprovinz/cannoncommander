package watch

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"slices"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const watchMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MODIFY | unix.IN_ATTRIB | unix.IN_CLOSE_WRITE |
	unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF |
	unix.IN_ONLYDIR | unix.IN_EXCL_UNLINK

// notifier owns the inotify descriptor. Its maps are guarded by the
// watcher's mu.
type notifier struct {
	w    *Watcher
	file *os.File
	fd   int
	// paths maps a watch to its folders. Two API paths that lead to the
	// same directory, through a link for example, share one watch.
	paths map[int][]string
	// moves keeps the first half of a rename until the second arrives.
	moves map[uint32]pendingMove
}

type pendingMove struct {
	folder string
	name   string
	at     time.Time
}

func newNotifier(w *Watcher) (*notifier, error) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	// A non-blocking descriptor lets the runtime poller wait on it, so
	// closing the file ends the reader.
	n := &notifier{w: w, file: os.NewFile(uintptr(fd), "inotify"), fd: fd, paths: map[int][]string{}, moves: map[uint32]pendingMove{}}
	w.done.Add(1)
	go n.run()
	return n, nil
}

func (n *notifier) add(real, folder string) (int, error) {
	wd, err := unix.InotifyAddWatch(n.fd, real, watchMask)
	if err != nil {
		return -1, &fs.PathError{Op: "inotify_add_watch", Path: real, Err: err}
	}
	n.paths[wd] = append(n.paths[wd], folder)
	return wd, nil
}

// remove drops the watch once no folder uses it any more. The caller
// holds mu and has already removed the folder.
func (n *notifier) remove(wd int, folder string) {
	n.paths[wd] = slices.DeleteFunc(n.paths[wd], func(p string) bool { return p == folder })
	for _, p := range n.paths[wd] {
		if f, ok := n.w.folders[p]; ok && f.watch == wd {
			return
		}
	}
	delete(n.paths, wd)
	_, _ = unix.InotifyRmWatch(n.fd, uint32(wd))
}

func (n *notifier) close() { _ = n.file.Close() }

func (n *notifier) run() {
	defer n.w.done.Done()
	buf := make([]byte, 64<<10)
	for {
		k, err := n.file.Read(buf)
		if err != nil {
			return
		}
		n.handle(buf[:k])
	}
}

func (n *notifier) handle(buf []byte) {
	w := n.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	for len(buf) >= unix.SizeofInotifyEvent {
		ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[0]))
		end := unix.SizeofInotifyEvent + int(ev.Len)
		if end > len(buf) {
			return
		}
		name := string(bytes.TrimRight(buf[unix.SizeofInotifyEvent:end], "\x00"))
		buf = buf[end:]
		n.event(int(ev.Wd), ev.Mask, ev.Cookie, name)
	}
	for c, m := range n.moves {
		if time.Since(m.at) > 2*time.Second {
			delete(n.moves, c)
		}
	}
}

func (n *notifier) event(wd int, mask, cookie uint32, name string) {
	w := n.w
	if mask&unix.IN_Q_OVERFLOW != 0 {
		for _, f := range w.folders {
			if f.watch >= 0 {
				w.note(f, func(b *batch) { b.reload = true })
			}
		}
		return
	}
	dir := mask&unix.IN_ISDIR != 0
	for _, f := range n.folders(wd) {
		switch {
		case mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF|unix.IN_IGNORED) != 0:
			// A moved folder keeps its watch under the new name, which then
			// differs from the path, so it is dropped and polled instead.
			f.watch, f.poll = -1, true
			if mask&unix.IN_MOVE_SELF != 0 {
				_, _ = unix.InotifyRmWatch(n.fd, uint32(wd))
			}
			w.note(f, func(b *batch) { b.gone = true })
		case name == "":
		case mask&unix.IN_MOVED_FROM != 0:
			n.moves[cookie] = pendingMove{folder: f.path, name: name, at: time.Now()}
			w.note(f, func(b *batch) { b.remove(name, dir) })
		case mask&unix.IN_MOVED_TO != 0:
			n.moved(f, cookie, name, dir)
		case mask&unix.IN_DELETE != 0:
			w.note(f, func(b *batch) { b.remove(name, dir) })
		default:
			w.note(f, func(b *batch) { b.add(name, dir) })
		}
	}
	if mask&unix.IN_IGNORED != 0 {
		delete(n.paths, wd)
	}
}

func (n *notifier) moved(f *folder, cookie uint32, name string, dir bool) {
	w := n.w
	m, ok := n.moves[cookie]
	delete(n.moves, cookie)
	if !ok {
		w.note(f, func(b *batch) { b.add(name, dir) })
		return
	}
	from := path.Join(m.folder, m.name)
	to := path.Join(f.path, name)
	w.note(f, func(b *batch) {
		if m.folder == f.path && b.removed[m.name] {
			b.rename(m.name, name, dir)
		} else {
			b.add(name, dir)
		}
		b.moves = append(b.moves, [2]string{from, to})
	})
}

func (n *notifier) folders(wd int) []*folder {
	var out []*folder
	for _, p := range n.paths[wd] {
		if f, ok := n.w.folders[p]; ok && f.watch == wd {
			out = append(out, f)
		}
	}
	return out
}

// needsPoll reports whether inotify misses changes in dir: on FUSE, where
// Unraid's shfs only reports what passes through it, and on network file
// systems, where other machines write.
func needsPoll(dir string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return true
	}
	switch uint32(st.Type) {
	case unix.FUSE_SUPER_MAGIC, unix.NFS_SUPER_MAGIC, unix.SMB_SUPER_MAGIC, unix.CIFS_SUPER_MAGIC, unix.SMB2_SUPER_MAGIC, unix.V9FS_MAGIC:
		return true
	}
	return false
}

func inoOf(info fs.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Ino
	}
	return 0
}
