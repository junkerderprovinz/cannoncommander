package web

import (
	"io/fs"
	"net/http"
	"net/url"
	"path"
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

// appPolicy keeps the app to what it loads itself, plus the document server,
// whose editor script runs in the page and talks to its server. Frames stay
// open because Collabora's address is not known here, and a frame runs nothing
// in this page.
func appPolicy() string {
	office := ""
	for _, origin := range onlyOfficeScriptSrcOrigins() {
		office += " " + origin
	}
	return "default-src 'self'; script-src 'self'" + office +
		"; style-src 'self' 'unsafe-inline' blob: https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data: blob:" +
		"; img-src 'self' blob: data:" + office + "; media-src 'self' blob:; connect-src 'self' blob: data:" + office +
		"; worker-src 'self' blob:; frame-src 'self' blob: http: https:; object-src 'none'; base-uri 'self'; frame-ancestors 'self'"
}

// rootFileHandler serves the files the browser asks for by a fixed name: the
// manifest of each language, its icons and the service worker. A new build keeps their names,
// so the browser has to ask again every time.
func rootFileHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(strings.TrimPrefix(r.URL.Path, settings.Config.Http.BaseURL)), "/")
	data, err := fs.ReadFile(assetFs, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	setContentType(w, name)
	_, _ = w.Write(data)
}

// sharedTooEarly answers a share from another app that arrives before the
// service worker runs, as on the first share after installing. The files
// are lost then, but the app still opens.
func sharedTooEarly(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, settings.Config.Http.BaseURL, http.StatusSeeOther)
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
	case ".webmanifest":
		w.Header().Set("Content-Type", "application/manifest+json")
	}
}
