package shares

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// maxOpenUploads bounds the part files visitors of one link can keep open,
// each of which holds space on the disk.
const maxOpenUploads = 20

var (
	errLocked    = v1.Errorf(http.StatusUnauthorized, "this link needs its password")
	errNoUploads = v1.Errorf(http.StatusForbidden, "this link takes no uploads")
	errNotHere   = v1.Errorf(http.StatusNotFound, "there is nothing at this place in the link")
)

// Public is what a visitor learns about a link. A locked link tells only
// that it is locked.
type Public struct {
	Slug    string     `json:"slug"`
	Locked  bool       `json:"locked"`
	Kind    string     `json:"kind,omitempty"`
	Name    string     `json:"name,omitempty"`
	Message string     `json:"message,omitempty"`
	Expires *time.Time `json:"expires,omitempty"`
	Uploads bool       `json:"uploads,omitempty"`
	// UploadRoom is how many bytes the link still takes, when it has an
	// upload limit.
	UploadRoom *int64 `json:"uploadRoom,omitempty"`
	// Size and MTime describe the file of a file link.
	Size  int64      `json:"size,omitempty"`
	MTime *time.Time `json:"mtime,omitempty"`
	// Count is the number of entries of a collection.
	Count int `json:"count,omitempty"`
}

// Item is one entry of a shared folder.
type Item struct {
	Name   string    `json:"name"`
	Dir    bool      `json:"dir"`
	Size   int64     `json:"size"`
	MTime  time.Time `json:"mtime"`
	Format string    `json:"format,omitempty"`
}

// Listing is a folder inside a link.
type Listing struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

type unlockRequest struct {
	Password string `json:"password"`
}

// Unlocked is the answer to the right password. The token goes into
// X-Share-Token or the token query, the cookie does the same for a browser.
type Unlocked struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

type uploadRequest struct {
	// Path is the folder inside the link, "/" by default.
	Path string `json:"path"`
	// Name may hold folders, as in "Urlaub/IMG_1.jpg", for a dropped folder.
	Name  string     `json:"name"`
	Size  int64      `json:"size"`
	MTime *time.Time `json:"mtime"`
	Rule  string     `json:"rule"`
}

// visit is a link a visitor opened, with the account whose rights it uses.
type visit struct {
	sh    *Share
	owner *users.User
}

func (s *Service) visit(w http.ResponseWriter, r *http.Request) (*visit, error) {
	slug := r.PathValue("slug")
	key := "slug|" + remoteIP(r)
	if wait, blocked := s.guard.blocked(key, maxUnknownSlugs); blocked {
		return nil, tooMany(w, wait)
	}
	var sh *Share
	var err error = v1.Errorf(http.StatusNotFound, "there is no such link")
	if slugPattern.MatchString(slug) {
		sh, err = s.one(r.Context(), `WHERE slug = ?`, slug)
	}
	if err != nil {
		var p *v1.Problem
		if errors.As(err, &p) && p.Status == http.StatusNotFound {
			s.guard.fail(key)
		}
		return nil, err
	}
	switch sh.State {
	case StateExpired:
		return nil, errExpired
	case StateExhausted:
		return nil, errUsedUp
	}
	// The link lives on the rights of its owner. An account that is gone,
	// or lost the right to share, takes its links with it.
	owner, err := UserByID(sh.OwnerID)
	if err != nil || !mayShare(owner) {
		return nil, errWithdrawn
	}
	return &visit{sh: sh, owner: owner}, nil
}

// open is visit for everything behind the password.
func (s *Service) open(w http.ResponseWriter, r *http.Request) (*visit, error) {
	v, err := s.visit(w, r)
	if err != nil {
		return nil, err
	}
	if v.sh.Password && !s.validToken(v.sh, tokenFrom(r, v.sh)) {
		return nil, errLocked
	}
	return v, nil
}

func tooMany(w http.ResponseWriter, wait time.Duration) error {
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	return v1.Errorf(http.StatusTooManyRequests, "too many tries, please wait a few minutes")
}

func (s *Service) publicGet(w http.ResponseWriter, r *http.Request) error {
	v, err := s.visit(w, r)
	if err != nil {
		return err
	}
	sh := v.sh
	if sh.Password && !s.validToken(sh, tokenFrom(r, sh)) {
		return v1.WriteJSON(w, r, http.StatusOK, Public{Slug: sh.Slug, Locked: true})
	}
	out := Public{Slug: sh.Slug, Kind: sh.Kind, Name: sh.Name, Message: sh.Message, Expires: sh.Expires, Uploads: sh.Uploads && sh.Kind == KindFolder}
	if out.Uploads && sh.UploadLimit > 0 {
		room := max(sh.UploadLimit-sh.Uploaded, 0)
		out.UploadRoom = &room
	}
	switch sh.Kind {
	case KindFile:
		sp, err := s.locate(v, "/")
		if err != nil {
			return err
		}
		info, err := os.Stat(sp.loc.Real)
		if err != nil {
			return err
		}
		mtime := info.ModTime().UTC()
		out.Size, out.MTime = info.Size(), &mtime
	case KindCollection:
		out.Count = len(sh.Paths)
	}
	s.opening(r, v)
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// opening counts a view and tells the owner, once per visitor in a while.
func (s *Service) opening(r *http.Request, v *visit) {
	sh := v.sh
	if !s.opened.first(sh.ID + "|" + remoteIP(r)) {
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `UPDATE share_links SET views = views + 1 WHERE id = ?`, sh.ID); err != nil {
		logger.Errorf("share %s: %v", sh.ID, err)
		return
	}
	sh.Views++
	if s.hub != nil {
		ev := struct {
			ID    string `json:"id"`
			Slug  string `json:"slug"`
			Name  string `json:"name"`
			Views int    `json:"views"`
		}{sh.ID, sh.Slug, sh.Name, sh.Views}
		if _, err := s.hub.PublishJSON(EventOpened, "", ev, v.owner.Username); err != nil {
			logger.Errorf("share %s: %v", sh.ID, err)
		}
	}
	if sh.Notify {
		n := unraid.Notice{Subject: "Link geöffnet", Description: "Jemand hat deinen Link zu " + sh.Name + " geöffnet"}
		if err := Notify(n); err != nil && !errors.Is(err, unraid.ErrWriteOff) {
			logger.Errorf("share %s: %v", sh.ID, err)
		}
	}
}

func (s *Service) publicUnlock(w http.ResponseWriter, r *http.Request) error {
	v, err := s.visit(w, r)
	if err != nil {
		return err
	}
	sh := v.sh
	var req unlockRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	until := time.Now().Add(unlockFor).UTC().Truncate(time.Second)
	if sh.Password {
		key := "pw|" + sh.ID + "|" + remoteIP(r)
		if wait, blocked := s.guard.blocked(key, maxWrongPasswords); blocked {
			return tooMany(w, wait)
		}
		if utils.CheckPwd(req.Password, sh.passwordHash) != nil {
			s.guard.fail(key)
			return v1.Errorf(http.StatusForbidden, "this password is not right")
		}
		s.guard.forget(key)
	}
	tok := s.token(sh, until)
	http.SetCookie(w, &http.Cookie{Name: cookiePrefix + sh.ID, Value: tok, Path: "/", Expires: until,
		HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteLaxMode})
	return v1.WriteJSON(w, r, http.StatusOK, Unlocked{Token: tok, Expires: until})
}

func (s *Service) publicList(w http.ResponseWriter, r *http.Request) error {
	v, err := s.open(w, r)
	if err != nil {
		return err
	}
	sp, err := s.locate(v, r.URL.Query().Get("path"))
	if err != nil {
		return err
	}
	out := Listing{Path: sp.path, Name: sp.name, Items: []Item{}}
	if sp.top {
		names := s.shownNames(v.sh.Paths)
		for i, p := range v.sh.Paths {
			loc, err := s.fs.Check(v.owner, p, ccfs.View)
			if err != nil {
				continue
			}
			if it, ok := itemAt(loc.Real, names[i]); ok {
				out.Items = append(out.Items, it)
			}
		}
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	loc, err := s.fs.Check(v.owner, sp.loc.Path, ccfs.View)
	if err != nil {
		return err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if it, ok := itemAt(loc.Real, sp.name); ok {
			out.Items = append(out.Items, it)
		}
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	dirents, err := os.ReadDir(loc.Real)
	if err != nil {
		return err
	}
	for _, d := range dirents {
		name := d.Name()
		if strings.HasPrefix(name, ".") || !s.fs.Visible(v.owner, loc, name) {
			continue
		}
		real := filepath.Join(loc.Real, name)
		if d.Type()&fs.ModeSymlink != 0 && !inside(sp.bound, real) {
			continue
		}
		if it, ok := itemAt(real, name); ok {
			out.Items = append(out.Items, it)
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

func itemAt(real, name string) (Item, bool) {
	info, err := os.Stat(real)
	if err != nil || !info.IsDir() && !info.Mode().IsRegular() {
		return Item{}, false
	}
	it := Item{Name: name, Dir: info.IsDir(), MTime: info.ModTime().UTC()}
	if !it.Dir {
		it.Size = info.Size()
		it.Format = fsapi.FormatOf(name)
	}
	return it, true
}

func (s *Service) publicRaw(w http.ResponseWriter, r *http.Request) error {
	v, err := s.open(w, r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	sp, err := s.locate(v, q.Get("path"))
	if err != nil {
		return err
	}
	download, _ := strconv.ParseBool(q.Get("download"))
	if sp.top {
		return s.archive(w, r, v, sp)
	}
	loc, err := s.fs.Check(v.owner, sp.loc.Path, ccfs.View|ccfs.Download)
	if err != nil {
		return err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return s.archive(w, r, v, sp)
	}
	if !info.Mode().IsRegular() {
		return errNotHere
	}
	// Looking at a file in the browser is free, and so is going on with a
	// download that broke off.
	if download && startsAtZero(r) {
		if err := s.countDownload(r.Context(), v.sh.ID); err != nil {
			return err
		}
	}
	return fsapi.ServeFile(w, r, loc.Real, sp.name, info, download)
}

func startsAtZero(r *http.Request) bool {
	rng := r.Header.Get("Range")
	return rng == "" || strings.HasPrefix(rng, "bytes=0-")
}

// archive sends a folder of the link, or the whole collection, as one zip.
// That counts as one download.
func (s *Service) archive(w http.ResponseWriter, r *http.Request, v *visit, sp spot) error {
	format := r.URL.Query().Get("format")
	switch format {
	case "":
		format = fsapi.FormatZip
	case fsapi.FormatZip, fsapi.FormatTgz:
	default:
		return v1.Errorf(http.StatusBadRequest, "format must be zip or tgz")
	}
	var entries []fsapi.ArchiveEntry
	if sp.top {
		names := s.shownNames(v.sh.Paths)
		for i, p := range v.sh.Paths {
			loc, err := s.fs.Check(v.owner, p, ccfs.View|ccfs.Download)
			if err != nil || s.fs.Follows(v.owner, loc, ccfs.View|ccfs.Download) != nil {
				continue
			}
			if _, err := os.Stat(loc.Real); err != nil {
				continue
			}
			entries = append(entries, fsapi.ArchiveEntry{Name: names[i], Loc: loc, Bound: loc.Real})
		}
	} else {
		loc, err := s.fs.Check(v.owner, sp.loc.Path, ccfs.View|ccfs.Download)
		if err != nil {
			return err
		}
		entries = append(entries, fsapi.ArchiveEntry{Name: sp.name, Loc: loc, Bound: sp.root.Real})
	}
	if len(entries) == 0 {
		return errNotHere
	}
	if err := s.countDownload(r.Context(), v.sh.ID); err != nil {
		return err
	}
	visible := func(dir ccfs.Location, name string) bool {
		return !strings.HasPrefix(name, ".") && s.fs.Visible(v.owner, dir, name)
	}
	fsapi.ServeArchive(w, r, s.fs, fsapi.ArchiveName(sp.name, format), entries, fsapi.ArchiveOptions{Format: format, Visible: visible})
	return nil
}

func (s *Service) publicUpload(w http.ResponseWriter, r *http.Request) error {
	v, err := s.uploader(w, r)
	if err != nil {
		return err
	}
	var req uploadRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	switch req.Rule {
	case "":
		req.Rule = jobs.Both
	case jobs.Both, jobs.Skip:
	default:
		return v1.Errorf(http.StatusBadRequest, "a link can add files but never replace them, so the rule is both or skip")
	}
	parts := strings.Split(req.Name, "/")
	for _, p := range parts {
		if !ccfs.ValidName(p) || strings.HasPrefix(p, ".") {
			return v1.Errorf(http.StatusBadRequest, "this file name is not allowed here")
		}
	}
	sp, err := s.locate(v, req.Path)
	if err != nil {
		return err
	}
	if info, err := os.Stat(sp.loc.Real); err != nil || !info.IsDir() {
		return errNotHere
	}
	target, err := s.fs.Resolve(path.Join(sp.loc.Path, req.Name))
	if err != nil {
		return err
	}
	if !inside(sp.bound, target.Real) {
		return errNotHere
	}

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	n, pending, err := s.files.PendingUploads(r.Context(), v.sh.ID)
	if err != nil {
		return err
	}
	if n >= maxOpenUploads {
		return v1.Errorf(http.StatusTooManyRequests, "many uploads are running through this link, please wait for some of them to finish")
	}
	// The body above can take as long as the visitor likes, so the first look
	// at the link may be old by now. Looking after the pending uploads means
	// a file landing in between counts twice for a moment instead of not at
	// all.
	if v, err = s.uploader(w, r); err != nil {
		return err
	}
	if v.sh.UploadLimit > 0 && v.sh.Uploaded+pending+req.Size > v.sh.UploadLimit {
		return v1.Errorf(http.StatusRequestEntityTooLarge, "this link takes no more than "+strconv.FormatInt(max(v.sh.UploadLimit-v.sh.Uploaded-pending, 0), 10)+" more bytes")
	}
	up, created, err := s.files.StartUpload(r.Context(), v.owner, v.sh.ID, fileops.UploadRequest{Path: target.Path, Size: req.Size, MTime: req.MTime, Rule: req.Rule})
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created && up.State == fileops.UploadOpen {
		status = http.StatusCreated
	}
	return fileops.WriteUpload(w, r, status, s.inLink(sp, up))
}

func (s *Service) publicUploadGet(w http.ResponseWriter, r *http.Request) error {
	v, err := s.uploader(w, r)
	if err != nil {
		return err
	}
	up, err := s.files.UploadState(r.Context(), v.owner, v.sh.ID, r.PathValue("id"))
	if err != nil {
		return err
	}
	return fileops.WriteUpload(w, r, http.StatusOK, s.inLink(s.rootSpot(v), up))
}

func (s *Service) publicUploadChunk(w http.ResponseWriter, r *http.Request) error {
	v, err := s.uploader(w, r)
	if err != nil {
		return err
	}
	offset, err := fileops.ChunkOffset(r)
	if err != nil {
		return err
	}
	root := s.rootSpot(v)
	landing := &fileops.Landing{
		// The body can outlast the link, or the owner's right to it.
		Allow: func() error {
			_, err := s.uploader(w, r)
			return err
		},
		Count: func(ctx context.Context, tx *sql.Tx, size int64) error {
			_, err := tx.ExecContext(ctx, `UPDATE share_links SET uploaded = uploaded + ? WHERE id = ?`, size, v.sh.ID)
			return err
		},
	}
	up, err := s.files.UploadChunk(r.Context(), v.owner, v.sh.ID, r.PathValue("id"), offset, r.Body, landing)
	if fileops.WriteUploadError(w, r, err, func(up fileops.Upload) fileops.Upload { return s.inLink(root, up) }) {
		return nil
	}
	if err != nil {
		return err
	}
	return fileops.WriteUpload(w, r, http.StatusOK, s.inLink(root, up))
}

func (s *Service) publicUploadCancel(w http.ResponseWriter, r *http.Request) error {
	v, err := s.uploader(w, r)
	if err != nil {
		return err
	}
	if err := s.files.CancelUpload(r.Context(), v.owner, v.sh.ID, r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// uploader is open for the links that take uploads, checked against the
// owner's right to create files there.
func (s *Service) uploader(w http.ResponseWriter, r *http.Request) (*visit, error) {
	v, err := s.open(w, r)
	if err != nil {
		return nil, err
	}
	if !v.sh.Uploads || v.sh.Kind != KindFolder || s.files == nil {
		return nil, errNoUploads
	}
	if _, err := s.fs.Check(v.owner, v.sh.Paths[0], ccfs.Create); err != nil {
		return nil, errNoUploads
	}
	return v, nil
}

// inLink shows an upload with its path inside the link, so a visitor
// never learns where the folder lies on the server.
func (s *Service) inLink(root spot, up fileops.Upload) fileops.Upload {
	rel := strings.TrimPrefix(up.Path, root.root.Path)
	up.Path = "/" + strings.TrimPrefix(rel, "/")
	return up
}

func (s *Service) rootSpot(v *visit) spot {
	root, _ := s.fs.Resolve(v.sh.Paths[0])
	return spot{root: root}
}

// spot is a place inside a link.
type spot struct {
	// path is the place as the visitor writes it, name what it is called.
	path, name string
	// top is the root of a collection, which is no folder on disk.
	top bool
	// loc is the place on the server, root the shared entry it lies in
	// and bound the real folder nothing may leave through a link.
	loc, root ccfs.Location
	bound     string
}

// locate maps a path inside a link to the server. Nothing below the
// shared entry may start with a dot, and no link may lead out of it.
func (s *Service) locate(v *visit, p string) (spot, error) {
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if strings.Contains(p, ccfs.ArchiveSep) {
		return spot{}, v1.Errorf(http.StatusBadRequest, "invalid path")
	}
	clean, err := ccfs.Clean(p)
	if err != nil {
		return spot{}, v1.Errorf(http.StatusBadRequest, "invalid path")
	}
	sh := v.sh
	sp := spot{path: clean, name: path.Base(clean)}
	segs := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if clean == "/" {
		segs, sp.name = nil, sh.Name
	}
	rootPath := sh.Paths[0]
	switch sh.Kind {
	case KindFile:
		if len(segs) > 1 || len(segs) == 1 && segs[0] != sh.Name {
			return spot{}, errNotHere
		}
		segs, sp.name = nil, sh.Name
	case KindCollection:
		if len(segs) == 0 {
			sp.top = true
			return sp, nil
		}
		i := slices.Index(s.shownNames(sh.Paths), segs[0])
		if i < 0 {
			return spot{}, errNotHere
		}
		rootPath, segs = sh.Paths[i], segs[1:]
	}
	for _, seg := range segs {
		if strings.HasPrefix(seg, ".") {
			return spot{}, errNotHere
		}
	}
	if sp.root, err = s.fs.Resolve(rootPath); err != nil || sp.root.Virtual() {
		return spot{}, errNotHere
	}
	if sp.loc, err = s.fs.Resolve(path.Join(append([]string{rootPath}, segs...)...)); err != nil || sp.loc.Virtual() {
		return spot{}, errNotHere
	}
	if sp.bound, err = filepath.EvalSymlinks(sp.root.Real); err != nil {
		return spot{}, errNotHere
	}
	if s.fs.Follows(v.owner, sp.loc, ccfs.View|ccfs.Download) != nil || !inside(sp.bound, sp.loc.Real) {
		return spot{}, errNotHere
	}
	return sp, nil
}

// inside reports whether real, with the links in its existing part
// followed, lies in the real folder bound.
func inside(bound, real string) bool {
	var tail []string
	cur := real
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			rel, err := filepath.Rel(bound, filepath.Join(append([]string{resolved}, tail...)...))
			return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
		}
		if _, lerr := os.Lstat(cur); lerr == nil {
			// A dangling link: following it on a write would create its
			// target, wherever that is.
			return false
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return false
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}
