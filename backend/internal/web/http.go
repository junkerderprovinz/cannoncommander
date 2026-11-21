package web

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// HttpResponse is the standard JSON error/success envelope.
type HttpResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

// ResponseWriterWrapper wraps http.ResponseWriter to capture status code and username.
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode  int
	WroteHeader bool
	User        string
}

// RenderJSON writes a JSON response, optionally gzip-compressed.
func RenderJSON(w http.ResponseWriter, r *http.Request, data interface{}, statusCode ...int) (int, error) {
	code := http.StatusOK
	if len(statusCode) > 0 && statusCode[0] != 0 {
		code = statusCode[0]
	}

	marsh, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	payloadSizeKB := len(marsh) / 1024
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if acceptsGzip(r) && payloadSizeKB > 10 {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(code)
		gz := gzip.NewWriter(w)
		defer gz.Close()
		if _, err := gz.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		w.WriteHeader(code)
		if _, err := w.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	return code, nil
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return ae != "" && strings.Contains(ae, "gzip")
}

// WriteHeader captures the status code and ensures it is only written once.
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	if !w.WroteHeader {
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		w.StatusCode = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
		w.WroteHeader = true
	}
}

// Write ensures WriteHeader is called before writing the body.
func (w *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !w.WroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher when the underlying writer supports it.
func (w *ResponseWriterWrapper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker so WebSocket upgrades (e.g. Vite HMR) work through LoggingMiddleware.
func (w *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

// SetUserInResponseWriter records the authenticated username on the wrapper when present.
func SetUserInResponseWriter(w http.ResponseWriter, user *users.User) {
	if wrappedWriter, ok := w.(*ResponseWriterWrapper); ok && user != nil {
		wrappedWriter.User = user.Username
	}
}

// GetRemoteIP resolves the client IP, honoring trusted proxy headers when configured.
func GetRemoteIP(r *http.Request) string {
	cfg := &settings.Config

	// Forwarded client headers are only honored when the socket peer is a
	// configured trusted proxy; otherwise a direct client could spoof its IP.
	trustedPeer := cfg.Http.TrustProxyHeaders && settings.IsTrustedProxyPeer(r)

	xff := r.Header.Get("X-Forwarded-For")
	if trustedPeer && xff != "" {
		// Without http.trustedProxies (Unix-socket listener) this returns the
		// rightmost XFF entry, the client as recorded by the proxy. With trusted proxies
		// configured, the walk skips proxy hops so multi-hop deployments
		// (CDN -> reverse proxy -> this server) resolve the real client IP,
		// keeping per-IP rate limits and auth lockouts per client, not per edge.
		if ip := settings.TrustedProxyClientIP(xff); ip != "" {
			return ip
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if trustedPeer && xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// GetScheme returns the request scheme (http or https).
func GetScheme(r *http.Request) string {
	return requestScheme(r)
}

// healthHandler returns a simple JSON health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HttpResponse{Message: "ok"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
