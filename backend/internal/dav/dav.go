// Package dav serves the sources over WebDAV with the users and rules of the
// app. Every access goes through the path resolver, so a client sees the same
// paths and gets the same answers as the panels.
package dav

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

const (
	methodPropfind  = "PROPFIND"
	methodProppatch = "PROPPATCH"
	methodMkcol     = "MKCOL"
	methodCopy      = "COPY"
	methodMove      = "MOVE"
	methodLock      = "LOCK"
	methodUnlock    = "UNLOCK"
)

// maxRequestXML caps the bodies of everything but PUT, which are short XML.
const maxRequestXML = 1 << 20

// maxLock is the longest a lock lasts. A client that goes away must not keep
// a file closed until the next restart, and clients refresh what they need.
const maxLock = time.Hour

// Identity is who a request signed in as. ReadOnly marks a read only API token.
type Identity struct {
	User     *users.User
	ReadOnly bool
}

// Options connect the handler to the rest of the server.
type Options struct {
	// Prefix is the URL path of the server, without a trailing slash.
	Prefix string
	// Enabled is asked on every request, so the switch works without a restart.
	Enabled func() bool
	// SignIn finds who a request signs in as, from the request alone. It must
	// not take a session cookie.
	SignIn func(http.ResponseWriter, *http.Request) (Identity, error)
}

// Handler is the WebDAV server.
type Handler struct {
	deps    v1.Deps
	opts    Options
	locks   webdav.LockSystem
	started time.Time
}

// New returns the handler for the sources in deps.
func New(deps v1.Deps, opts Options) *Handler {
	return &Handler{deps: deps, opts: opts, locks: webdav.NewMemLS(), started: time.Now()}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.opts.Enabled() {
		http.NotFound(w, r)
		return
	}
	// Browsers attach Basic credentials they remember to any request a page
	// makes, while WebDAV clients do not send this header at all.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "none" {
		http.Error(w, "this address is for WebDAV clients", http.StatusForbidden)
		return
	}
	id, err := h.opts.SignIn(w, r)
	if err != nil {
		refuse(w, err)
		return
	}
	if !known(r.Method) {
		w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, MKCOL, COPY, MOVE, PROPFIND, PROPPATCH, LOCK, UNLOCK")
		http.Error(w, "this method is not part of the server", http.StatusMethodNotAllowed)
		return
	}
	if id.ReadOnly && !reading(r.Method) {
		http.Error(w, "this API token may only read", http.StatusForbidden)
		return
	}
	s := newSession(h, id.User, r)
	if r.Method == http.MethodOptions {
		s.options(w, r)
		return
	}
	if err := s.permit(r); err != nil {
		answerProblem(w, r, err)
		return
	}
	h.serve(s, w, r)
}

// serve hands a permitted request to the WebDAV library and announces what it
// changed.
func (h *Handler) serve(s *session, w http.ResponseWriter, r *http.Request) {
	hdr := w.Header()
	// A page from the share would otherwise run with the app's origin and session.
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	hdr.Set("X-Content-Type-Options", "nosniff")
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		hdr.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(r.URL.Path)}))
	case http.MethodPut:
		r.Body = &trackedBody{r.Body, s}
		if !s.mtime.IsZero() {
			hdr.Set("X-OC-Mtime", "accepted")
		}
	case methodLock:
		capLock(r)
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestXML)
	default:
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestXML)
	}
	out := &response{ResponseWriter: w, s: s}
	lib := &webdav.Handler{
		Prefix:     h.opts.Prefix,
		FileSystem: s,
		LockSystem: h.locks,
		Logger: func(r *http.Request, err error) {
			if err != nil {
				logger.Debugf("webdav %s %s: %v", r.Method, r.URL.Path, err)
			}
		},
	}
	lib.ServeHTTP(out, r)
	s.finish(out.status)
}

// strip returns a URL path below the server without the prefix.
func (h *Handler) strip(p string) (string, bool) {
	rest, ok := strings.CutPrefix(p, h.opts.Prefix)
	return rest, ok && (rest == "" || rest[0] == '/')
}

func known(method string) bool {
	switch method {
	case http.MethodOptions, http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete,
		methodMkcol, methodCopy, methodMove, methodPropfind, methodProppatch, methodLock, methodUnlock:
		return true
	}
	return false
}

func reading(method string) bool {
	switch method {
	case http.MethodOptions, http.MethodGet, http.MethodHead, methodPropfind:
		return true
	}
	return false
}

func refuse(w http.ResponseWriter, err error) {
	var wait *accounts.TooManyTries
	if errors.As(err, &wait) {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Wait.Seconds())+1))
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="CannonCommander", charset="UTF-8"`)
	http.Error(w, "sign in with your name and password or an API token", http.StatusUnauthorized)
}

// statusOf picks the answer to err, as the API would, and tells a full disk
// apart from other failures.
func statusOf(err error) int {
	if errors.Is(err, syscall.ENOSPC) {
		return http.StatusInsufficientStorage
	}
	return v1.ProblemFor(err).Status
}

func answerProblem(w http.ResponseWriter, r *http.Request, err error) {
	status := statusOf(err)
	if status >= http.StatusInternalServerError {
		logger.Errorf("webdav %s %s: %v", r.Method, r.URL.Path, err)
	}
	http.Error(w, http.StatusText(status), status)
}

// capLock keeps the Timeout header of a LOCK within maxLock.
func capLock(r *http.Request) {
	most := int64(maxLock / time.Second)
	first, _, _ := strings.Cut(r.Header.Get("Timeout"), ",")
	secs, ok := strings.CutPrefix(strings.TrimSpace(first), "Second-")
	if n, err := strconv.ParseInt(secs, 10, 64); ok && err == nil && n > 0 && n <= most {
		return
	}
	r.Header.Set("Timeout", "Second-"+strconv.FormatInt(most, 10))
}

// response replaces the status the library picks for any failed write. It
// answers 405 to nearly everything, which tells a client nothing.
type response struct {
	http.ResponseWriter
	s      *session
	status int
}

func (o *response) WriteHeader(status int) {
	if status >= http.StatusBadRequest && o.s.failure != nil {
		status = statusOf(o.s.failure)
	}
	o.status = status
	o.ResponseWriter.WriteHeader(status)
}

func (o *response) Write(b []byte) (int, error) {
	if o.status == 0 {
		o.status = http.StatusOK
	}
	return o.ResponseWriter.Write(b)
}

// trackedBody remembers a broken upload. The library closes the file whether
// the body arrived or not, and only the file can keep it from taking the name.
type trackedBody struct {
	io.ReadCloser
	s *session
}

func (b *trackedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		b.s.fail(err)
	}
	return n, err
}
