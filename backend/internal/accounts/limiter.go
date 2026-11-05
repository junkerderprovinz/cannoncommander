package accounts

import (
	"crypto/sha256"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// limiter counts failed attempts per key and blocks a key that failed too
// often within the window, so passwords and codes cannot be guessed. The key ""
// stands for an address nobody knows and is never counted.
type limiter struct {
	max    int
	window time.Duration

	mu    sync.Mutex
	fails map[string][]time.Time
	swept time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *limiter) blocked(key string) bool {
	if key == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) >= l.max
}

func (l *limiter) fail(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep()
	l.fails[key] = append(l.recent(key), time.Now())
}

// sweep forgets the keys whose failures left the window, since most of them
// never come back. Call with l.mu held.
func (l *limiter) sweep() {
	now := time.Now()
	if now.Sub(l.swept) < l.window {
		return
	}
	l.swept = now
	for key := range l.fails {
		l.recent(key)
	}
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// recent drops the failures that left the window. Call with l.mu held.
func (l *limiter) recent(key string) []time.Time {
	cut := time.Now().Add(-l.window)
	list := l.fails[key]
	i := 0
	for i < len(list) && list[i].Before(cut) {
		i++
	}
	list = list[i:]
	if len(list) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = list
	return list
}

// pairKey is the key for one name from one address. It is a hash because the
// name comes straight from the request and can be megabytes long.
func pairKey(name, ip string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name) + "|" + clientNet(ip)))
	return string(sum[:16])
}

// clientNet is what counts as one address. A home connection gets a whole IPv6
// network and can move around in it at will.
func clientNet(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is6() || addr.Is4In6() {
		return ip
	}
	network, err := addr.Prefix(64)
	if err != nil {
		return ip
	}
	return network.String()
}

var warnUnnamedProxy sync.Once

// addressKey is the key for everything coming from the address of r. Behind a
// proxy the config does not name, that address is the proxy, and counting it
// would let one stranger lock out everyone. Such a request is not counted by
// address, which leaves the count per name.
func addressKey(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != ""
	if forwarded && !(settings.Config.Http.TrustProxyHeaders && settings.IsTrustedProxyPeer(r)) {
		warnUnnamedProxy.Do(func() {
			logger.Warning("Requests come through a proxy that is not set up as trusted. Set http.trustProxyHeaders and http.trustedProxies so wrong sign ins count per client.")
		})
		return ""
	}
	return clientNet(login.ClientIP(r))
}

// wait returns how long key stays blocked.
func (l *limiter) wait(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.recent(key)
	if len(list) < l.max {
		return 0
	}
	return time.Until(list[len(list)-l.max].Add(l.window))
}
