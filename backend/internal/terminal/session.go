package terminal

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// session is one open terminal.
type session struct {
	user, hash string

	last atomic.Int64
	once sync.Once
	// ending is closed once something asked to end the terminal, and code
	// and reason say why. They are set before it closes.
	ending chan struct{}
	code   websocket.StatusCode
	reason string
}

func newSession(user, hash string) *session {
	return &session{user: user, hash: hash, ending: make(chan struct{})}
}

// end asks the terminal to close. The first reason given is the one it
// closes with.
func (s *session) end(code websocket.StatusCode, reason string) {
	s.once.Do(func() {
		s.code, s.reason = code, reason
		close(s.ending)
	})
}

func (s *session) touch() { s.last.Store(time.Now().UnixNano()) }

func (s *session) lastInput() time.Time { return time.Unix(0, s.last.Load()) }

// pool holds the open terminals, a few per user.
type pool struct {
	mu     sync.Mutex
	limit  int
	open   map[*session]struct{}
	closed bool
	wg     sync.WaitGroup
}

func newPool(limit int) *pool {
	return &pool{limit: limit, open: map[*session]struct{}{}}
}

func (p *pool) count(user string) int {
	n := 0
	for s := range p.open {
		if s.user == user {
			n++
		}
	}
	return n
}

func (p *pool) full(user string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count(user) >= p.limit
}

func (p *pool) add(s *session) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errStopping
	}
	if p.count(s.user) >= p.limit {
		return errLimit
	}
	p.open[s] = struct{}{}
	p.wg.Add(1)
	return nil
}

func (p *pool) remove(s *session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.open[s]; ok {
		delete(p.open, s)
		p.wg.Done()
	}
}

// Close ends every terminal and waits until their shells are gone, ten
// seconds at most, so a stop of the server never hangs on one.
func (p *pool) Close() {
	p.mu.Lock()
	p.closed = true
	for s := range p.open {
		s.end(websocket.StatusGoingAway, "stopping")
	}
	p.mu.Unlock()
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}

// size is the terminal in characters.
type size struct{ cols, rows uint16 }

func sizeOf(cols, rows string) size {
	c, _ := strconv.Atoi(cols)
	r, _ := strconv.Atoi(rows)
	return size{clamp(c, 80), clamp(r, 24)}
}

// clamp keeps n between 1 and 1000 and takes def for anything below.
func clamp(n int, def uint16) uint16 {
	switch {
	case n < 1:
		return def
	case n > 1000:
		return 1000
	}
	return uint16(n)
}
