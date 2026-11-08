package watch

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// notifyMask is what Explorer reacts to: names, sizes, times and attributes.
const notifyMask = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME |
	windows.FILE_NOTIFY_CHANGE_SIZE | windows.FILE_NOTIFY_CHANGE_LAST_WRITE |
	windows.FILE_NOTIFY_CHANGE_ATTRIBUTES | windows.FILE_NOTIFY_CHANGE_CREATION

// notifier asks ReadDirectoryChangesW for each watched folder, with one
// completion port for all of them. Its maps are guarded by the watcher's
// mu, apart from what only run touches.
type notifier struct {
	w    *Watcher
	port windows.Handle
	next int
	dirs map[int]*dirWatch
	// closing keeps a watch whose handle is closed until its last read
	// completes, since the system writes into the buffer until then.
	closing  map[int]*dirWatch
	stopping bool
}

type dirWatch struct {
	id    int
	real  string
	h     windows.Handle
	ov    windows.Overlapped
	buf   [64 << 10]byte
	paths []string
	// oldName holds the first half of a rename until the second arrives.
	oldName string
}

func newNotifier(w *Watcher) (*notifier, error) {
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		return nil, err
	}
	n := &notifier{w: w, port: port, next: 1, dirs: map[int]*dirWatch{}, closing: map[int]*dirWatch{}}
	w.done.Add(1)
	go n.run()
	return n, nil
}

func (n *notifier) add(real, folder string) (int, error) {
	for id, d := range n.dirs {
		if strings.EqualFold(d.real, real) {
			d.paths = append(d.paths, folder)
			return id, nil
		}
	}
	name, err := windows.UTF16PtrFromString(real)
	if err != nil {
		return -1, err
	}
	h, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return -1, &fs.PathError{Op: "CreateFile", Path: real, Err: err}
	}
	d := &dirWatch{id: n.next, real: real, h: h, paths: []string{folder}}
	if _, err := windows.CreateIoCompletionPort(h, n.port, uintptr(d.id), 0); err != nil {
		windows.CloseHandle(h)
		return -1, err
	}
	if err := d.read(); err != nil {
		windows.CloseHandle(h)
		return -1, &fs.PathError{Op: "ReadDirectoryChanges", Path: real, Err: err}
	}
	n.next++
	n.dirs[d.id] = d
	return d.id, nil
}

func (d *dirWatch) read() error {
	return windows.ReadDirectoryChanges(d.h, &d.buf[0], uint32(len(d.buf)), false, notifyMask, nil, &d.ov, 0)
}

// remove drops the watch once no folder uses it any more. The caller holds
// mu and has already removed the folder.
func (n *notifier) remove(id int, folder string) {
	d, ok := n.dirs[id]
	if !ok {
		return
	}
	d.paths = deleteString(d.paths, folder)
	for _, p := range d.paths {
		if f, ok := n.w.folders[p]; ok && f.watch == id {
			return
		}
	}
	n.drop(d)
}

// drop closes the folder of a watch whose read is still pending, which then
// completes as cancelled.
func (n *notifier) drop(d *dirWatch) {
	delete(n.dirs, d.id)
	n.closing[d.id] = d
	windows.CloseHandle(d.h)
}

// release closes the folder of a watch that has no read pending and hands
// its folders to the poller.
func (n *notifier) release(d *dirWatch, folders []*folder) {
	delete(n.dirs, d.id)
	windows.CloseHandle(d.h)
	for _, f := range folders {
		f.watch, f.poll = -1, true
	}
}

func (n *notifier) close() {
	n.w.mu.Lock()
	for _, d := range n.dirs {
		n.drop(d)
	}
	n.stopping = true
	n.w.mu.Unlock()
	_ = windows.PostQueuedCompletionStatus(n.port, 0, 0, nil)
}

func (n *notifier) run() {
	defer n.w.done.Done()
	defer windows.CloseHandle(n.port)
	for {
		var got uint32
		var key uintptr
		var ov *windows.Overlapped
		err := windows.GetQueuedCompletionStatus(n.port, &got, &key, &ov, windows.INFINITE)
		w := n.w
		w.mu.Lock()
		if ov != nil {
			n.complete(int(key), got, err)
		}
		done := n.stopping && len(n.closing) == 0
		w.mu.Unlock()
		if done {
			return
		}
	}
}

// complete handles one finished read. The caller holds mu.
func (n *notifier) complete(id int, got uint32, err error) {
	if d, ok := n.closing[id]; ok {
		delete(n.closing, d.id)
		return
	}
	d, ok := n.dirs[id]
	if !ok {
		return
	}
	folders := n.folders(d)
	if err != nil {
		// The folder went away or became unreadable. A folder that is
		// deleted stays until the last handle on it closes, so the check
		// comes after the release.
		n.release(d, folders)
		if !exists(d.real) {
			for _, f := range folders {
				n.w.note(f, func(b *batch) { b.gone = true })
			}
		}
		return
	}
	if got == 0 {
		// More happened than the buffer could hold.
		for _, f := range folders {
			n.w.note(f, func(b *batch) { b.reload = true })
		}
	} else {
		n.events(d, folders, d.buf[:got])
	}
	if err := d.read(); err != nil {
		n.release(d, folders)
	}
}

func (n *notifier) events(d *dirWatch, folders []*folder, buf []byte) {
	for off := 0; off+12 <= len(buf); {
		info := (*windows.FileNotifyInformation)(unsafe.Pointer(&buf[off]))
		size := int(info.FileNameLength) / 2
		name := filepath.ToSlash(windows.UTF16ToString(unsafe.Slice(&info.FileName, size)))
		action := info.Action
		if strings.Contains(name, "/") {
			name = path.Base(name)
		}
		for _, f := range folders {
			n.event(d, f, action, name)
		}
		if action == windows.FILE_ACTION_RENAMED_OLD_NAME {
			d.oldName = name
		} else if action == windows.FILE_ACTION_RENAMED_NEW_NAME {
			d.oldName = ""
		}
		if info.NextEntryOffset == 0 {
			break
		}
		off += int(info.NextEntryOffset)
	}
}

func (n *notifier) event(d *dirWatch, f *folder, action uint32, name string) {
	w := n.w
	isDir := func() bool {
		info, err := os.Lstat(filepath.Join(d.real, name))
		return err == nil && info.IsDir()
	}
	switch action {
	case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_MODIFIED:
		dir := isDir()
		w.note(f, func(b *batch) { b.add(name, dir) })
	case windows.FILE_ACTION_REMOVED:
		// What is gone cannot say what it was, and a stale listing of a
		// folder that never was costs nothing.
		w.note(f, func(b *batch) { b.remove(name, true) })
	case windows.FILE_ACTION_RENAMED_OLD_NAME:
		w.note(f, func(b *batch) { b.remove(name, true) })
	case windows.FILE_ACTION_RENAMED_NEW_NAME:
		dir, from := isDir(), d.oldName
		w.note(f, func(b *batch) {
			if from != "" && b.removed[from] {
				b.rename(from, name, dir)
				b.moves = append(b.moves, [2]string{path.Join(f.path, from), path.Join(f.path, name)})
			} else {
				b.add(name, dir)
			}
		})
	}
}

func (n *notifier) folders(d *dirWatch) []*folder {
	var out []*folder
	for _, p := range d.paths {
		if f, ok := n.w.folders[p]; ok && f.watch == d.id {
			out = append(out, f)
		}
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !errors.Is(err, fs.ErrNotExist)
}

func deleteString(list []string, s string) []string {
	out := list[:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// needsPoll reports whether the system misses changes in dir: on a network
// drive or share, where other computers write.
func needsPoll(dir string) bool {
	root := filepath.VolumeName(dir) + `\`
	if strings.HasPrefix(root, `\\`) {
		return true
	}
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return true
	}
	return windows.GetDriveType(p) == windows.DRIVE_REMOTE
}

func inoOf(fs.FileInfo) uint64 { return 0 }
