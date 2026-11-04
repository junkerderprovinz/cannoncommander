package dav

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

const writePerms = ccfs.Create | ccfs.Modify | ccfs.Delete

// closedName reports the names a client never gets to see: what the app keeps
// for itself, and names the API paths would read as a way into an archive.
func closedName(name string) bool {
	return name == trash.DirName || fileops.IsPart(name) || strings.Contains(name, ccfs.ArchiveSep)
}

// rooted gives a name from the library its leading slash, which it lacks at
// the root and in names it joins itself.
func rooted(name string) string { return "/" + strings.TrimPrefix(name, "/") }

// closedPath reports whether any part of the API path p has a closed name.
func closedPath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if closedName(seg) {
			return true
		}
	}
	return false
}

// reach checks that the user holds perm on name and, when a symlink leads
// elsewhere, on the place it leads to.
func (s *session) reach(name string, perm ccfs.Perm) (ccfs.Location, error) {
	fsys := s.h.deps.FS
	loc, err := fsys.Check(s.u, rooted(name), perm)
	if err != nil {
		return loc, err
	}
	if closedPath(loc.Path) {
		return loc, notThere(perm)
	}
	if loc.Virtual() {
		return loc, nil
	}
	target, err := fsys.Followed(loc)
	if err != nil {
		return loc, err
	}
	if target.Path != loc.Path {
		if closedPath(target.Path) {
			return loc, notThere(perm)
		}
		if _, err := fsys.Check(s.u, target.Path, perm); err != nil {
			return loc, err
		}
	}
	return loc, nil
}

func notThere(perm ccfs.Perm) error {
	if perm&writePerms != 0 {
		return ccfs.ErrForbidden
	}
	return ccfs.ErrNotFound
}

// writable checks the right that writing to name takes: Modify where an entry
// exists and Create where it is new. It also returns what is there.
func (s *session) writable(name string) (ccfs.Location, fs.FileInfo, error) {
	loc, err := s.h.deps.FS.Resolve(rooted(name))
	if err != nil {
		return loc, nil, err
	}
	old, err := os.Stat(loc.Real)
	perm := ccfs.Modify
	if err != nil {
		old, perm = nil, ccfs.Create
	}
	loc, err = s.reach(name, perm)
	return loc, old, err
}

// removable checks that name may go away. A source itself stays.
func (s *session) removable(name string, perm ccfs.Perm) (ccfs.Location, error) {
	loc, err := s.reach(name, perm)
	if err != nil {
		return loc, err
	}
	if loc.Index == "/" {
		return loc, ccfs.ErrForbidden
	}
	return loc, nil
}

// wholeTree checks perm on everything below name. A rule can name one entry
// deep inside a folder, and taking the folder along would take that entry past
// its rule. Links are not followed, they go as links.
func (s *session) wholeTree(name string, perm ccfs.Perm) error {
	fsys := s.h.deps.FS
	top, err := fsys.Resolve(rooted(name))
	if err != nil {
		return err
	}
	err = filepath.WalkDir(top.Real, func(real string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if strings.Contains(d.Name(), ccfs.ArchiveSep) {
			// The resolver cannot name it, so no rule can either.
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(top.Real, real)
		if err != nil {
			return err
		}
		_, err = fsys.Check(s.u, path.Join(top.Path, filepath.ToSlash(rel)), perm)
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// rights checks what method takes on name, as far as that is known without
// looking at the rest of the request.
func (s *session) rights(method, name string) error {
	var err error
	switch method {
	case methodPropfind:
		_, err = s.reach(name, ccfs.View)
	case http.MethodGet, http.MethodHead, methodCopy:
		_, err = s.reach(name, ccfs.View|ccfs.Download)
	case http.MethodPut, methodLock, methodUnlock:
		_, _, err = s.writable(name)
	case methodMkcol:
		_, err = s.reach(name, ccfs.Create)
	case methodProppatch:
		_, err = s.reach(name, ccfs.Modify)
	case http.MethodDelete:
		_, err = s.removable(name, ccfs.Delete)
	case methodMove:
		if _, err = s.removable(name, ccfs.View|ccfs.Delete); err != nil {
			_, err = s.removable(name, ccfs.Modify)
		}
	}
	return err
}

// permit decides a request before the library sees it. The library answers a
// refusal with 404 or 405, and by the time it gets to the file system a MOVE
// has already deleted its target.
func (s *session) permit(r *http.Request) error {
	name, ok := s.h.strip(r.URL.Path)
	if !ok {
		return ccfs.ErrNotFound
	}
	if err := s.rights(r.Method, name); err != nil {
		return err
	}
	switch r.Method {
	case methodPropfind:
		// A walk over a whole share would keep every disk busy for one request.
		if depth := r.Header.Get("Depth"); depth != "0" && depth != "1" {
			return v1.Errorf(http.StatusForbidden, "PROPFIND needs a Depth of 0 or 1")
		}
	case http.MethodPut:
		// The body would replace the whole file.
		if r.Header.Get("Content-Range") != "" {
			return v1.Errorf(http.StatusBadRequest, "partial uploads are not supported")
		}
		if loc, err := s.h.deps.FS.Resolve(rooted(name)); err == nil {
			if info, err := os.Stat(loc.Real); err == nil && info.IsDir() {
				return v1.Errorf(http.StatusMethodNotAllowed, "a folder cannot be overwritten")
			}
		}
	case http.MethodDelete:
		return s.wholeTree(name, ccfs.Delete)
	case methodCopy, methodMove:
		return s.permitTransfer(r, name)
	}
	return nil
}

// permitTransfer checks both ends of a COPY or MOVE. A Destination the library
// would refuse by itself goes on to it.
func (s *session) permitTransfer(r *http.Request, name string) error {
	dest, err := url.Parse(r.Header.Get("Destination"))
	if err != nil {
		return nil
	}
	dst, ok := s.h.strip(dest.Path)
	if !ok || dst == "" {
		return nil
	}
	from, err := ccfs.Clean(rooted(name))
	if err != nil {
		return err
	}
	to, err := ccfs.Clean(rooted(dst))
	if err != nil {
		return err
	}
	// The library deletes an existing target first, which here would take the
	// source with it.
	if from == to || strings.HasPrefix(to, from+"/") || strings.HasPrefix(from, to+"/") {
		return ccfs.ErrForbidden
	}
	move := r.Method == methodMove
	overwrite := r.Header.Get("Overwrite") != "F"
	if move {
		// The library takes a missing header as F here, unlike for COPY.
		overwrite = r.Header.Get("Overwrite") == "T"
		perm := moveRight(from, to)
		if _, err := s.removable(from, perm); err != nil {
			return err
		}
		if err := s.wholeTree(from, perm); err != nil {
			return err
		}
	}
	target, err := s.reach(to, ccfs.Create)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target.Real); err == nil && overwrite {
		if _, err := s.removable(to, ccfs.Delete); err != nil {
			return err
		}
		if err := s.wholeTree(to, ccfs.Delete); err != nil {
			return err
		}
	}
	if !move {
		s.copied = &labels.Move{From: from, To: to}
	}
	return nil
}

// moveRight is what the API asks for: a rename changes the entry, a move to
// another folder takes it away from its old one.
func moveRight(from, to string) ccfs.Perm {
	if path.Dir(from) == path.Dir(to) {
		return ccfs.Modify
	}
	return ccfs.View | ccfs.Delete
}

// options answers with the methods this user can use here, so a client does
// not offer what would only fail.
func (s *session) options(w http.ResponseWriter, r *http.Request) {
	name, ok := s.h.strip(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	candidates := []string{http.MethodPut, methodMkcol, methodLock}
	if info, err := s.Stat(s.ctx, name); err == nil {
		candidates = []string{methodPropfind, http.MethodDelete, methodCopy, methodMove, methodProppatch, methodLock, methodUnlock}
		if !info.IsDir() {
			candidates = append(candidates, http.MethodGet, http.MethodHead, http.MethodPut)
		}
	}
	allow := []string{http.MethodOptions}
	for _, m := range candidates {
		if s.rights(m, name) == nil {
			allow = append(allow, m)
		}
	}
	hdr := w.Header()
	hdr.Set("Allow", strings.Join(allow, ", "))
	hdr.Set("DAV", "1, 2")
	hdr.Set("MS-Author-Via", "DAV")
	w.WriteHeader(http.StatusOK)
}
