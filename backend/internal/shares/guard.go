package shares

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const (
	// unlockFor is how long an unlocked link stays open in one browser.
	unlockFor = 12 * time.Hour
	// Wrong passwords per link and address before it has to wait, and
	// unknown addresses one client may try, against guessing.
	maxWrongPasswords = 5
	maxUnknownSlugs   = 30
	guardWindow       = 5 * time.Minute
	// openedAgain is how long a visitor counts as one view, so reloading
	// the page neither counts nor notifies again.
	openedAgain  = 30 * time.Minute
	cookiePrefix = "cc_share_"
)

// token proves the password of a share for a while. It is bound to the
// password hash, so setting a new password closes every unlocked browser.
func (s *Service) token(sh *Share, until time.Time) string {
	exp := make([]byte, 8)
	binary.BigEndian.PutUint64(exp, uint64(until.Unix()))
	return base64.RawURLEncoding.EncodeToString(exp) + "." + base64.RawURLEncoding.EncodeToString(s.mac(sh, exp))
}

func (s *Service) mac(sh *Share, exp []byte) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(sh.ID))
	m.Write([]byte{0})
	m.Write([]byte(sh.passwordHash))
	m.Write([]byte{0})
	m.Write(exp)
	return m.Sum(nil)[:16]
}

func (s *Service) validToken(sh *Share, tok string) bool {
	expPart, macPart, ok := strings.Cut(tok, ".")
	if !ok {
		return false
	}
	exp, err := base64.RawURLEncoding.DecodeString(expPart)
	if err != nil || len(exp) != 8 {
		return false
	}
	sum, err := base64.RawURLEncoding.DecodeString(macPart)
	if err != nil || !hmac.Equal(sum, s.mac(sh, exp)) {
		return false
	}
	return time.Now().Unix() < int64(binary.BigEndian.Uint64(exp))
}

// tokenFrom finds the unlock token in a header for apps, in the query for
// plain links to files, or in the cookie the unlock set.
func tokenFrom(r *http.Request, sh *Share) string {
	if t := r.Header.Get("X-Share-Token"); t != "" {
		return t
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	if c, err := r.Cookie(cookiePrefix + sh.ID); err == nil {
		return c.Value
	}
	return ""
}

// guard counts failures per key in fixed windows.
type guard struct {
	mu   sync.Mutex
	hits map[string]*window
}

type window struct {
	n     int
	until time.Time
}

func newGuard() *guard { return &guard{hits: map[string]*window{}} }

// blocked reports whether key used up its tries, and how long it waits.
func (g *guard) blocked(key string, max int) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w := g.hits[key]
	now := time.Now()
	if w == nil || now.After(w.until) {
		return 0, false
	}
	return w.until.Sub(now), w.n >= max
}

func (g *guard) fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if len(g.hits) > 10000 {
		for k, w := range g.hits {
			if now.After(w.until) {
				delete(g.hits, k)
			}
		}
	}
	w := g.hits[key]
	if w == nil || now.After(w.until) {
		w = &window{until: now.Add(guardWindow)}
		g.hits[key] = w
	}
	w.n++
}

func (g *guard) forget(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.hits, key)
}

// opens remembers who opened which link lately.
type opens struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newOpens() *opens { return &opens{seen: map[string]time.Time{}} }

// first reports whether this visit is the first of key in a while.
func (o *opens) first(key string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := time.Now()
	if len(o.seen) > 10000 {
		for k, t := range o.seen {
			if now.Sub(t) > openedAgain {
				delete(o.seen, k)
			}
		}
	}
	if t, ok := o.seen[key]; ok && now.Sub(t) < openedAgain {
		return false
	}
	o.seen[key] = now
	return true
}

// remoteIP is the visitor's address. Forwarded headers count only from a
// proxy the admin trusts, or anyone could pick a fresh address per guess.
func remoteIP(r *http.Request) string {
	cfg := &settings.Config
	if cfg.Http.TrustProxyHeaders && settings.IsTrustedProxyPeer(r) {
		if ip := settings.TrustedProxyClientIP(r.Header.Get("X-Forwarded-For")); ip != "" {
			return ip
		}
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// secure reports whether the visitor reached the server over https, so the
// unlock cookie is only ever sent that way.
func secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	cfg := &settings.Config
	return cfg.Http.TrustProxyHeaders && settings.IsTrustedProxyPeer(r) && r.Header.Get("X-Forwarded-Proto") == "https"
}
