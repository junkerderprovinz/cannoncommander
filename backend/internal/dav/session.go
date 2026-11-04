package dav

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
)

// session is one request's view of the files, for the library to work on. The
// library runs a request on one goroutine, so it needs no lock.
type session struct {
	h   *Handler
	u   *users.User
	ctx context.Context

	// failure is what went wrong while writing. It keeps a part file from
	// taking its name and sets the status of the answer.
	failure error
	// seen holds what a listing already checked, because the library looks
	// at every entry again one by one.
	seen map[string]seenEntry
	// mtime is the time a client asked for with X-OC-Mtime.
	mtime time.Time
	// keepTime is set for a COPY, where a new file takes the time of lastRead.
	keepTime bool
	lastRead fs.FileInfo
	// copied carries the labels along once a COPY went through.
	copied *labels.Move
	// changes collects what happened per folder, to announce at the end.
	changes map[string]*watch.Change
}

type seenEntry struct {
	loc  ccfs.Location
	info fs.FileInfo
}

func newSession(h *Handler, u *users.User, r *http.Request) *session {
	s := &session{h: h, u: u, ctx: r.Context(), seen: map[string]seenEntry{}, changes: map[string]*watch.Change{}, keepTime: r.Method == methodCopy}
	if raw := r.Header.Get("X-OC-Mtime"); raw != "" && r.Method == http.MethodPut {
		// A time the disk cannot store would fail an upload that went well.
		if secs, err := strconv.ParseFloat(raw, 64); err == nil && secs > 0 && secs < 1<<36 {
			s.mtime = time.Unix(int64(secs), 0)
		}
	}
	return s
}

func (s *session) fail(err error) error {
	if s.failure == nil {
		s.failure = err
	}
	return err
}

// osError turns a refusal into what the library looks for.
func osError(err error) error {
	switch {
	case errors.Is(err, ccfs.ErrNotFound):
		return os.ErrNotExist
	case errors.Is(err, fs.ErrNotExist):
		return err
	}
	return os.ErrPermission
}

// Stat implements webdav.FileSystem.
func (s *session) Stat(_ context.Context, name string) (fs.FileInfo, error) {
	if clean, err := ccfs.Clean(rooted(name)); err == nil {
		if e, ok := s.seen[clean]; ok {
			return e.info, nil
		}
	}
	loc, err := s.reach(name, ccfs.View)
	if err != nil {
		return nil, osError(err)
	}
	return s.stat(loc)
}

func (s *session) stat(loc ccfs.Location) (fs.FileInfo, error) {
	if loc.Virtual() {
		return virtualInfo{name: path.Base(loc.Path), at: s.h.started}, nil
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return nil, err
	}
	return entry{info, path.Base(loc.Path)}, nil
}

// OpenFile implements webdav.FileSystem. The library opens every entry of a
// listing to look at it, so a file only opens on disk when it gets read.
func (s *session) OpenFile(_ context.Context, name string, flag int, _ fs.FileMode) (webdav.File, error) {
	if flag&os.O_CREATE != 0 {
		return s.create(name)
	}
	var loc ccfs.Location
	var info fs.FileInfo
	clean, err := ccfs.Clean(rooted(name))
	if err != nil {
		return nil, osError(err)
	}
	if e, ok := s.seen[clean]; ok && flag == os.O_RDONLY {
		loc, info = e.loc, e.info
	} else {
		perm := ccfs.View
		if flag != os.O_RDONLY {
			perm = ccfs.Modify
		}
		if loc, err = s.reach(name, perm); err != nil {
			return nil, osError(err)
		}
		if info, err = s.stat(loc); err != nil {
			return nil, err
		}
	}
	s.lastRead = info
	return &file{s: s, loc: loc, info: info}, nil
}

// create starts a file under a part name. It takes the real name when it is
// closed whole. The part name is new each time, so two uploads of one file or
// the rest of a crash do not get in each other's way.
func (s *session) create(name string) (webdav.File, error) {
	loc, old, err := s.writable(name)
	if err != nil {
		return nil, osError(err)
	}
	target, err := s.h.deps.FS.Followed(loc)
	if err != nil {
		return nil, osError(err)
	}
	tmp, err := partName(target.Real)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return nil, err
	}
	p := &part{File: f, s: s, tmp: tmp, target: target, old: old, mtime: s.mtime}
	if s.keepTime && s.lastRead != nil {
		p.mtime = s.lastRead.ModTime()
	}
	return p, nil
}

func partName(real string) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(real), "."+filepath.Base(real)+"."+hex.EncodeToString(b[:])+fileops.PartSuffix), nil
}

// Mkdir implements webdav.FileSystem.
func (s *session) Mkdir(_ context.Context, name string, _ fs.FileMode) error {
	loc, err := s.reach(name, ccfs.Create)
	if err != nil {
		return osError(err)
	}
	if err := os.Mkdir(loc.Real, 0o777); err != nil {
		return err
	}
	fschange.Created(loc.Path)
	s.added(loc.Path)
	return nil
}

// RemoveAll implements webdav.FileSystem. It deletes for good, as WebDAV
// clients expect. Their temporary files would otherwise fill the trash.
func (s *session) RemoveAll(_ context.Context, name string) error {
	loc, err := s.removable(name, ccfs.Delete)
	if err != nil {
		return osError(err)
	}
	info, err := os.Lstat(loc.Real)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(loc.Real); err != nil {
		return s.fail(err)
	}
	fschange.Removed(loc.Path)
	if err := labels.Removed(context.WithoutCancel(s.ctx), s.h.deps, loc.Path); err != nil {
		logger.Errorf("labels: %v", err)
	}
	s.removed(loc.Path, info.IsDir())
	return nil
}

// Rename implements webdav.FileSystem.
func (s *session) Rename(_ context.Context, oldName, newName string) error {
	from, err := s.removable(oldName, moveRight(rooted(oldName), rooted(newName)))
	if err != nil {
		return osError(err)
	}
	to, err := s.reach(newName, ccfs.Create)
	if err != nil {
		return osError(err)
	}
	info, err := os.Lstat(from.Real)
	if err != nil {
		return err
	}
	err = os.Rename(from.Real, to.Real)
	if ccfs.CrossDevice(err) {
		err = moveAcross(from.Real, to.Real)
	}
	if err != nil {
		return s.fail(err)
	}
	fschange.Moved(from.Path, to.Path)
	if err := labels.Moved(context.WithoutCancel(s.ctx), s.h.deps, labels.Move{From: from.Path, To: to.Path}); err != nil {
		logger.Errorf("labels: %v", err)
	}
	s.moved(from.Path, to.Path, info.IsDir())
	return nil
}

// moveAcross copies to a part name, so that the target only shows up whole,
// and deletes the source once the target stands.
func moveAcross(from, to string) error {
	tmp, err := partName(to)
	if err != nil {
		return err
	}
	if err := fileutils.CopyFile(from, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, to); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return os.RemoveAll(from)
}

func (s *session) change(dir string) *watch.Change {
	c, ok := s.changes[dir]
	if !ok {
		c = &watch.Change{Path: dir, Paths: []string{dir}}
		s.changes[dir] = c
	}
	return c
}

func (s *session) added(p string) {
	c := s.change(path.Dir(p))
	c.Names = append(c.Names, path.Base(p))
}

func (s *session) removed(p string, dir bool) {
	c := s.change(path.Dir(p))
	c.Removed = append(c.Removed, path.Base(p))
	if dir {
		c.Paths = append(c.Paths, p)
	}
}

func (s *session) moved(from, to string, dir bool) {
	if path.Dir(from) != path.Dir(to) {
		s.removed(from, dir)
		s.added(to)
		return
	}
	c := s.change(path.Dir(from))
	c.Renamed = append(c.Renamed, watch.Rename{From: path.Base(from), To: path.Base(to)})
	if dir {
		c.Paths = append(c.Paths, from)
	}
}

// finish tells the panels and the index what the request changed. The watcher
// may see the same, but on network mounts only after its next look.
func (s *session) finish(status int) {
	if s.copied != nil && status < http.StatusMultipleChoices {
		if err := labels.Copied(context.WithoutCancel(s.ctx), s.h.deps, *s.copied); err != nil {
			logger.Errorf("labels: %v", err)
		}
	}
	dirs := make([]string, 0, len(s.changes))
	for dir := range s.changes {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	for _, dir := range dirs {
		c := s.changes[dir]
		if _, err := s.h.deps.Hub.PublishJSON(watch.EventChanged, hub.FolderTopic(dir), c); err != nil {
			logger.Errorf("webdav: publish %s: %v", dir, err)
		}
		if s.h.deps.Index == nil {
			continue
		}
		for _, p := range c.Paths {
			if loc, err := s.h.deps.FS.Resolve(p); err == nil && !loc.Virtual() {
				s.h.deps.Index.Refresh(loc.Source, loc.Index)
			}
		}
	}
}

// file is an entry opened for reading or listing.
type file struct {
	s    *session
	loc  ccfs.Location
	info fs.FileInfo
	f    *os.File
}

func (f *file) open() error {
	if f.f != nil {
		return nil
	}
	var err error
	f.f, err = os.Open(f.loc.Real)
	return err
}

func (f *file) Read(b []byte) (int, error) {
	if err := f.open(); err != nil {
		return 0, f.s.fail(err)
	}
	n, err := f.f.Read(b)
	if err != nil && !errors.Is(err, io.EOF) {
		f.s.fail(err)
	}
	return n, err
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.f.Seek(offset, whence)
}

func (f *file) Write([]byte) (int, error) { return 0, fs.ErrPermission }

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *file) Close() error {
	if f.f == nil {
		return nil
	}
	return f.f.Close()
}

// Readdir returns what the user may see inside the folder. The library only
// ever asks for all of it.
func (f *file) Readdir(int) ([]fs.FileInfo, error) {
	s, fsys := f.s, f.s.h.deps.FS
	var out []fs.FileInfo
	add := func(name string, info fs.FileInfo) {
		loc, err := fsys.Resolve(path.Join(f.loc.Path, name))
		if err == nil {
			s.seen[loc.Path] = seenEntry{loc, info}
			out = append(out, info)
		}
	}
	if f.loc.Virtual() {
		for _, name := range fsys.VirtualChildren(f.loc.Path) {
			if !fsys.Visible(s.u, f.loc, name) {
				continue
			}
			if info, err := s.Stat(s.ctx, path.Join(f.loc.Path, name)); err == nil {
				add(name, info)
			}
		}
		return out, nil
	}
	entries, err := os.ReadDir(f.loc.Real)
	if err != nil {
		return nil, err
	}
	// Below a real folder only links can lead anywhere else, so plain entries
	// can skip following them, which adds up in a big folder.
	target, err := fsys.Followed(f.loc)
	if err != nil {
		return nil, err
	}
	direct := target.Path == f.loc.Path
	for _, d := range entries {
		name := d.Name()
		if closedName(name) || !fsys.Visible(s.u, f.loc, name) {
			continue
		}
		if info, err := f.listed(d, direct); err == nil {
			add(name, info)
		}
	}
	return out, nil
}

func (f *file) listed(d fs.DirEntry, direct bool) (fs.FileInfo, error) {
	if direct && d.Type()&fs.ModeSymlink == 0 {
		info, err := d.Info()
		if err != nil {
			return nil, err
		}
		return entry{info, d.Name()}, nil
	}
	return f.s.Stat(f.s.ctx, path.Join(f.loc.Path, d.Name()))
}

// part is a file being written under its part name.
type part struct {
	webdav.File
	s      *session
	tmp    string
	target ccfs.Location
	old    fs.FileInfo
	mtime  time.Time
}

func (p *part) Write(b []byte) (int, error) {
	if err := p.s.ctx.Err(); err != nil {
		return 0, p.s.fail(err)
	}
	n, err := p.File.Write(b)
	if err != nil {
		p.s.fail(err)
	}
	return n, err
}

// Stat settles the time the file is going to have, since the library builds
// the ETag from this before it closes the file.
func (p *part) Stat() (fs.FileInfo, error) {
	info, err := p.File.Stat()
	if err != nil {
		return nil, err
	}
	if p.mtime.IsZero() {
		p.mtime = info.ModTime()
	}
	return timedInfo{info, p.mtime}, nil
}

// Close gives a complete file its name, or drops it.
func (p *part) Close() error {
	s := p.s
	err := cmp.Or(p.File.Close(), s.failure, s.ctx.Err())
	if err == nil && p.old != nil {
		// A replaced file stays what it was to the other users of the share.
		_ = os.Chmod(p.tmp, p.old.Mode().Perm())
		fileops.Chown(p.tmp, p.old)
	}
	if err == nil && !p.mtime.IsZero() {
		err = os.Chtimes(p.tmp, p.mtime, p.mtime)
	}
	if err == nil {
		err = os.Rename(p.tmp, p.target.Real)
	}
	if err != nil {
		_ = os.Remove(p.tmp)
		return s.fail(err)
	}
	if p.old == nil {
		fschange.Created(p.target.Path)
	}
	s.added(p.target.Path)
	return nil
}

// entry is a file or folder on disk under the name its path gives it, since
// the library builds the paths of a listing from that and a source need not be
// named like its mount. Its content type comes from the name as well. The
// library would read the file to find out, which wakes a sleeping disk and
// takes a right a listing does not need.
type entry struct {
	fs.FileInfo
	name string
}

func (e entry) Name() string { return e.name }

func (e entry) ContentType(context.Context) (string, error) {
	if t := mime.TypeByExtension(path.Ext(e.name)); t != "" {
		return t, nil
	}
	return "application/octet-stream", nil
}

// timedInfo is a file that is about to get the time at.
type timedInfo struct {
	fs.FileInfo
	at time.Time
}

func (t timedInfo) ModTime() time.Time { return t.at }

// virtualInfo is a folder that only exists to lead to the sources.
type virtualInfo struct {
	name string
	at   time.Time
}

func (v virtualInfo) Name() string       { return v.name }
func (v virtualInfo) Size() int64        { return 0 }
func (v virtualInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (v virtualInfo) ModTime() time.Time { return v.at }
func (v virtualInfo) IsDir() bool        { return true }
func (v virtualInfo) Sys() any           { return nil }
