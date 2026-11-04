package meta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Limits of one fs/meta request. The list asks for the rows on screen, a
// sort by a metadata column for the whole folder.
const (
	maxMetaPaths   = 5000
	maxMetaColumns = 100
	metaBudget     = 25 * time.Second
)

// Routes binds fs/meta and fs/page, whose contract is part of the fs
// package.
func Routes(r *v1.Router, s *Service) {
	h := handlers{fs: r.Deps().FS, s: s}
	r.Handle("POST /fs/meta", h.meta)
	r.Handle("GET /fs/page", h.page)
}

type handlers struct {
	fs *ccfs.Resolver
	s  *Service
}

type metaRequest struct {
	Paths   []string `json:"paths"`
	Columns []string `json:"columns"`
}

type metaResponse struct {
	Items map[string]Values `json:"items"`
}

func (h handlers) meta(w http.ResponseWriter, r *http.Request) error {
	var req metaRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.Paths) > maxMetaPaths || len(req.Columns) > maxMetaColumns {
		return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("ask for at most %d paths and %d columns at once", maxMetaPaths, maxMetaColumns))
	}
	var cols []string
	for _, c := range req.Columns {
		if Known(c) && !containsField(cols, c) {
			cols = append(cols, c)
		}
	}
	out := metaResponse{Items: map[string]Values{}}
	if len(cols) == 0 {
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	u := v1.UserFrom(r)
	files := make([]File, 0, len(req.Paths))
	asked := map[string]string{}
	for _, p := range req.Paths {
		f, ok := h.file(u, p)
		if !ok {
			continue
		}
		if _, dup := asked[f.Path]; dup {
			continue
		}
		asked[f.Path] = p
		files = append(files, f)
	}
	ctx, cancel := context.WithTimeout(r.Context(), metaBudget)
	defer cancel()
	for p, v := range h.s.Columns(ctx, files, cols) {
		if len(v) > 0 {
			out.Items[asked[p]] = v
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// file looks a path up for fs/meta. Paths the user may not see, that are
// gone or that are not real files and folders are skipped without a word,
// the list asks for whole screens at once.
func (h handlers) file(u *users.User, p string) (File, bool) {
	loc, err := h.fs.Check(u, p, ccfs.View)
	if err != nil || loc.Virtual() || loc.InArchive() || ccfs.Contained(loc) != nil {
		return File{}, false
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return File{}, false
	}
	return File{Path: loc.Path, Real: loc.Real, Size: info.Size(), MTime: info.ModTime(), Dir: info.IsDir()}, true
}

func (h handlers) page(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	if q.Get("path") == "" {
		return v1.Errorf(http.StatusBadRequest, "path is required")
	}
	page, err := intParam(q.Get("page"), 1, 1, 100000)
	if err != nil {
		return v1.Errorf(http.StatusBadRequest, "page is a number from 1")
	}
	width, err := intParam(q.Get("width"), DefaultPageWidth, 64, MaxPageWidth)
	if err != nil {
		return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("width is a number from 64 to %d", MaxPageWidth))
	}
	loc, err := h.fs.Check(v1.UserFrom(r), q.Get("path"), ccfs.Download)
	if err != nil {
		return err
	}
	if loc.Virtual() || loc.InArchive() {
		return v1.Errorf(http.StatusUnsupportedMediaType, "this has no pages to show")
	}
	if err := ccfs.Contained(loc); err != nil {
		return err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	ext := extOf(path.Base(loc.Path))
	if !info.Mode().IsRegular() || !(pagedExts[ext] || officeExts[ext]) {
		return v1.Errorf(http.StatusUnsupportedMediaType, "this file has no pages to show")
	}
	etag := fmt.Sprintf(`"%x-%x-p%d-w%d"`, info.Size(), info.ModTime().UnixNano(), page, width)
	hdr := w.Header()
	hdr.Set("ETag", etag)
	// With the file's mtime in v the URL changes whenever the page might,
	// so the browser may keep it.
	if q.Get("v") != "" {
		hdr.Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		hdr.Set("Cache-Control", "private, no-cache")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	data, err := h.s.Page(r.Context(), loc.Real, page, width)
	if err != nil {
		hdr.Del("ETag")
		hdr.Del("Cache-Control")
		switch {
		case errors.Is(err, ErrNoRenderer):
			return v1.Errorf(http.StatusServiceUnavailable, "pages can not be drawn because MuPDF is missing")
		case errors.Is(err, ErrNoPage):
			return v1.Errorf(http.StatusNotFound, "this page does not exist or can not be drawn")
		}
		return err
	}
	hdr.Set("Content-Type", "image/png")
	hdr.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	return err
}

func intParam(s string, def, lo, hi int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return 0, errors.New("out of range")
	}
	return n, nil
}
