//go:build desktop

package desktop

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Options is how the desktop app runs the server.
type Options struct {
	// Key is the secret the window proves itself with. Only a request that
	// carries it in its cookie reaches the server.
	Key string
	// Port is the loopback port the server listens on.
	Port int
	// Home is the folder the panels open when nobody picked one, a path on
	// disk.
	Home string
}

var (
	mu   sync.RWMutex
	app  App
	opts Options
)

// Use connects the server to the desktop app before the server starts.
func Use(a App, o Options) {
	mu.Lock()
	defer mu.Unlock()
	app, opts = a, o
}

// Current returns the desktop app, or nil in the container.
func Current() App {
	mu.RLock()
	defer mu.RUnlock()
	return app
}

// Active reports whether the server runs inside the desktop app.
func Active() bool { return Current() != nil }

// Home returns the folder the panels open in the desktop app, or "".
func Home() string {
	mu.RLock()
	defer mu.RUnlock()
	return opts.Home
}

// CookieName holds the key in the window.
const CookieName = "cc_desktop"

// EnterPath is where the window starts: it trades the key in the query for
// the cookie and goes on to the interface, with the folders to open, if any.
const EnterPath = "/desktop/enter"

// Guard lets through only what the window sends. The server listens on
// loopback, where every program on the computer and every web page in a
// browser could reach it. The Host check stops a page that rebinds its own
// name to 127.0.0.1, the SameSite cookie stops one that posts a form here,
// and the key keeps out other programs.
func Guard(next http.Handler) http.Handler {
	mu.RLock()
	key, port := opts.Key, opts.Port
	mu.RUnlock()
	if key == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host, port) {
			http.Error(w, "this address is for the CannonCommander window", http.StatusMisdirectedRequest)
			return
		}
		if r.URL.Path == EnterPath {
			if !same(r.URL.Query().Get("key"), key) {
				http.Error(w, "this address is for the CannonCommander window", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name: CookieName, Value: key, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			to := "/"
			if open := r.URL.Query().Get("open"); open != "" {
				to += "?" + url.Values{"open": {open}}.Encode()
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		c, err := r.Cookie(CookieName)
		if err != nil || !same(c.Value, key) {
			http.Error(w, "this address is for the CannonCommander window", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func same(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// loopbackHost reports whether host names this server by a loopback address
// or by localhost, with its port.
func loopbackHost(host string, port int) bool {
	name, p, err := net.SplitHostPort(host)
	if err != nil || p != strconv.Itoa(port) {
		return false
	}
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}
