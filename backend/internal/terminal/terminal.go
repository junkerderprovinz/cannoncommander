// Package terminal opens a shell in a folder for an admin and relays it over
// a WebSocket.
package terminal

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

//go:embed openapi.yaml
var contract []byte

// Gate is what the terminal asks the web server about a request.
type Gate interface {
	// SameOrigin refuses a request a browser sent from another origin, by
	// the rules the server holds writes to. A WebSocket opens with a GET,
	// which those rules let pass.
	SameOrigin(r *http.Request) error
	// Session returns the hash of the session token r signed in with. ok is
	// false for an API token and for a request without a session.
	Session(r *http.Request) (hash string, ok bool)
	// Alive reports whether the session with this hash still signs in.
	Alive(hash string) bool
	// APIToken reports whether r signed in with an API token.
	APIToken(r *http.Request) bool
	// Open reports whether the server lets everybody in without signing in.
	Open() bool
}

var gate Gate

// UseGate connects the terminal to the web server. The server calls it once
// before it registers the routes.
func UseGate(g Gate) { gate = g }

const (
	maxPerUser = 4
	// maxMessage bounds one message from the browser, which splits a long
	// paste into smaller ones.
	maxMessage = 64 << 10
	pingEvery  = 30 * time.Second
)

const (
	closeNormal = websocket.StatusNormalClosure
	closeIdle   = websocket.StatusCode(4001)
	closeAccess = websocket.StatusCode(4003)
)

var (
	errOff         = v1.Coded(http.StatusForbidden, "terminalOff", "the terminal is switched off")
	errToken       = v1.Coded(http.StatusForbidden, "terminalToken", "an API token cannot open a terminal")
	errCrossOrigin = v1.Coded(http.StatusForbidden, "crossOrigin", "the request came from another origin")
	errSignIn      = v1.Coded(http.StatusUnauthorized, "signInNeeded", "sign in to open a terminal")
	errNotAFolder  = v1.Coded(http.StatusBadRequest, "notAFolder", "a terminal opens in a folder on disk")
	errLimit       = v1.Coded(http.StatusTooManyRequests, "terminalLimit", "too many terminals are open")
	errStopping    = v1.Coded(http.StatusServiceUnavailable, "stopping", "the server is stopping")
)

// Routes registers the contract and the handler.
func Routes(r *v1.Router) {
	r.Contract("terminal", contract)
	h := newHandlers(r.Deps(), gate)
	system.OnShutdown(h.pool.Close)
	bind(r, h)
}

func bind(r *v1.Router, h *handlers) {
	r.Handle("GET /terminal", h.open)
}

type handlers struct {
	deps    v1.Deps
	gate    Gate
	enabled func(ctx context.Context) (bool, error)
	record  func(r *http.Request, u *users.User, e activitydb.Entry)
	pool    *pool
	// idle is how long a terminal stays open without typing, every how
	// often access is checked again while it is open.
	idle, every time.Duration
}

func newHandlers(deps v1.Deps, g Gate) *handlers {
	return &handlers{
		deps: deps,
		gate: g,
		enabled: func(ctx context.Context) (bool, error) {
			a, err := system.Settings(ctx, deps)
			return a.Terminal, err
		},
		record: func(r *http.Request, u *users.User, e activitydb.Entry) {
			activity.RecordUser(r, &activity.Actor{User: u}, e)
		},
		pool:  newPool(maxPerUser),
		idle:  30 * time.Minute,
		every: 5 * time.Second,
	}
}

func (h *handlers) open(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	hash, err := h.admit(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	loc, err := h.folder(u, q.Get("path"))
	if err != nil {
		return err
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		if h.pool.full(u.Username) {
			return errLimit
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	s := newSession(u.Username, hash)
	if err := h.pool.add(s); err != nil {
		return err
	}
	defer h.pool.remove(s)
	t, err := start(loc.Real, sizeOf(q.Get("cols"), q.Get("rows")))
	if err != nil {
		return err
	}
	// admit already held the origin to the host the browser asked for, which
	// knows about a proxy in front, while Accept would only see the proxy.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		t.stop()
		return nil
	}
	began := time.Now()
	h.record(r, u, activitydb.Entry{EventType: activitydb.EventTerminalOpen, Path: loc.Path})
	logger.Infof("terminal opened for %s in %s", u.Username, loc.Path)
	h.serve(r.Context(), conn, t, s)
	h.record(r, u, activitydb.Entry{
		EventType: activitydb.EventTerminalClose,
		Path:      loc.Path,
		Details:   activitydb.Details{DurationMs: time.Since(began).Milliseconds()},
	})
	logger.Infof("terminal closed for %s in %s: %s", u.Username, loc.Path, s.reason)
	return nil
}

// admit checks everything about r but the folder and returns the hash of
// its session, which is empty on a server without sign in.
func (h *handlers) admit(r *http.Request) (string, error) {
	if err := h.gate.SameOrigin(r); err != nil {
		return "", errCrossOrigin
	}
	if h.gate.APIToken(r) {
		return "", errToken
	}
	hash, ok := h.gate.Session(r)
	if !ok && !h.gate.Open() {
		return "", errSignIn
	}
	on, err := h.enabled(r.Context())
	if err != nil {
		return "", err
	}
	if !on {
		return "", errOff
	}
	return hash, nil
}

// folder finds the folder on disk where the terminal starts. Its links are
// followed, so the rules judge the place the shell really lands in.
func (h *handlers) folder(u *users.User, p string) (ccfs.Location, error) {
	loc, err := h.deps.FS.Landed(u, p, ccfs.View)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() {
		return loc, errNotAFolder
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, err
	}
	if !info.IsDir() {
		return loc, errNotAFolder
	}
	return loc, nil
}

// serve relays between the browser and the shell until one of them ends or
// access does, and leaves no process of the shell behind.
func (h *handlers) serve(parent context.Context, conn *websocket.Conn, t tty, s *session) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	conn.SetReadLimit(maxMessage)
	s.touch()
	drained := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		defer close(drained)
		relayOut(ctx, conn, t, s)
	})
	wg.Go(func() { relayIn(ctx, conn, t, s) })
	wg.Go(func() { h.watch(ctx, conn, s) })
	select {
	case <-s.ending:
	case <-t.exited():
		// The last lines the shell printed are still on their way.
		select {
		case <-drained:
		case <-time.After(500 * time.Millisecond):
		}
		s.end(closeNormal, "exited")
	}
	t.stop()
	_ = conn.Close(s.code, s.reason)
	cancel()
	wg.Wait()
}

func relayOut(ctx context.Context, conn *websocket.Conn, t tty, s *session) {
	buf := make([]byte, 32<<10)
	for {
		n, err := t.Read(buf)
		if n > 0 {
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				s.end(websocket.StatusGoingAway, "gone")
				return
			}
		}
		if err != nil {
			s.end(closeNormal, "exited")
			return
		}
	}
}

// relayIn hands what is typed to the shell. A text message is a new size.
func relayIn(ctx context.Context, conn *websocket.Conn, t tty, s *session) {
	for {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			s.end(closeNormal, "closed")
			return
		}
		s.touch()
		if kind == websocket.MessageText {
			var sz struct{ Cols, Rows int }
			if json.Unmarshal(data, &sz) == nil {
				_ = t.resize(size{clamp(sz.Cols, 80), clamp(sz.Rows, 24)})
			}
			continue
		}
		if _, err := t.Write(data); err != nil {
			return
		}
	}
}

// watch ends the terminal once it sits idle or its user may no longer have
// it, and pings the browser so a proxy keeps the connection.
func (h *handlers) watch(ctx context.Context, conn *websocket.Conn, s *session) {
	tick := time.NewTicker(h.every)
	defer tick.Stop()
	pinged := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ending:
			return
		case <-tick.C:
		}
		if why := h.lost(ctx, s); why != "" {
			s.end(closeAccess, why)
			return
		}
		if time.Since(s.lastInput()) > h.idle {
			s.end(closeIdle, "idle")
			return
		}
		if time.Since(pinged) < pingEvery {
			continue
		}
		pctx, done := context.WithTimeout(ctx, 15*time.Second)
		err := conn.Ping(pctx)
		done()
		if err != nil {
			s.end(websocket.StatusGoingAway, "gone")
			return
		}
		pinged = time.Now()
	}
}

// lost names why the user of s may no longer have a terminal, or is empty.
// A user that cannot be looked up counts as unchanged: the database closes
// before the terminals do when the server stops, and a deleted user's
// sessions end with it anyway.
func (h *handlers) lost(ctx context.Context, s *session) string {
	if s.hash != "" && !h.gate.Alive(s.hash) {
		return "signedOut"
	}
	if u, err := h.deps.Users(s.user); err == nil && !u.Permissions.Admin {
		return "denied"
	}
	if on, err := h.enabled(ctx); err != nil || !on {
		return "off"
	}
	return ""
}

// tty is a shell on a pseudo terminal.
type tty interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	resize(size) error
	// exited is closed once the shell has ended.
	exited() <-chan struct{}
	// stop ends the shell with every job it started and waits for it.
	stop()
}
