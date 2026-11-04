package web

import (
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// indexHandler answers every path the router does not know with the app,
// which then routes in the browser.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	page, err := fs.ReadFile(assetFs, "index.html")
	if err != nil {
		http.Error(w, "the app is missing from this build", http.StatusNotFound)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache, private, max-age=0")
	h.Set("Content-Security-Policy", appPolicy())
	_, _ = w.Write(page)
}

// appPolicy keeps the app to its own scripts, plus the document server
// whose editor script the office viewer loads.
func appPolicy() string {
	policy := "script-src 'self'"
	for _, origin := range onlyOfficeScriptSrcOrigins() {
		policy += " " + origin
	}
	return policy + "; worker-src blob:"
}

func onlyOfficeScriptSrcOrigins() []string {
	oo := settings.Config.Integrations.OnlyOffice
	var origins []string
	for _, raw := range []string{oo.Url, oo.InternalUrl} {
		if origin := cspOriginFromURL(raw); origin != "" && !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins
}

func cspOriginFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func setContentType(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	}
}
