package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestEveryUnknownPageOpensTheApp(t *testing.T) {
	saved, savedBase, savedOffice := assetFs, settings.Config.Http.BaseURL, settings.Config.Integrations.OnlyOffice.Url
	t.Cleanup(func() {
		assetFs, settings.Config.Http.BaseURL, settings.Config.Integrations.OnlyOffice.Url = saved, savedBase, savedOffice
	})
	assetFs = fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	settings.Config.Http.BaseURL = "/"
	settings.Config.Integrations.OnlyOffice.Url = "https://office.tower.lan/"
	router := http.NewServeMux()
	configureHTTPRouter(router)

	get := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	rec := get("GET", "/files/mnt/user/filme")
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" {
		t.Fatalf("page: %d %s", rec.Code, rec.Body)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' https://office.tower.lan;") {
		t.Fatalf("policy %q", csp)
	}
	if rec := get("POST", "/files"); rec.Code != http.StatusNotFound {
		t.Fatalf("POST on a page: %d", rec.Code)
	}
	for _, path := range []string{"/api/auth/login", "/api/resources", "/api/users"} {
		if rec := get("GET", path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
	if rec := get("GET", "/health"); rec.Code != http.StatusOK {
		t.Fatalf("health: %d", rec.Code)
	}
}

func TestTheAppCanBeInstalledAndTakeShares(t *testing.T) {
	saved, savedBase := assetFs, settings.Config.Http.BaseURL
	t.Cleanup(func() { assetFs, settings.Config.Http.BaseURL = saved, savedBase })
	assetFs = fstest.MapFS{
		"index.html":           {Data: []byte("<html>app</html>")},
		"manifest.webmanifest": {Data: []byte(`{"name":"CannonCommander"}`)},
		"sw.js":                {Data: []byte("addEventListener('fetch', () => {})")},
		"icons/app-512.png":    {Data: []byte("png")},
		"icons/favicon.svg":    {Data: []byte("<svg/>")},
	}
	settings.Config.Http.BaseURL = "/"
	router := http.NewServeMux()
	configureHTTPRouter(router)
	do := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}

	for path, kind := range map[string]string{
		"/manifest.webmanifest": "application/manifest+json",
		"/sw.js":                "application/javascript; charset=utf-8",
		"/icons/app-512.png":    "image/png",
		"/icons/favicon.svg":    "image/svg+xml",
	} {
		rec := do("GET", path)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != kind || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d %q %q", path, rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"))
		}
	}
	if rec := do("GET", "/icons/../index.html"); rec.Code == http.StatusOK && rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("an icon path reached another file: %q", rec.Header().Get("Content-Type"))
	}
	if rec := do("GET", "/icons/missing.png"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing icon: %d", rec.Code)
	}
	if csp := do("GET", "/").Header().Get("Content-Security-Policy"); !strings.Contains(csp, "worker-src 'self' blob:") {
		t.Fatalf("the policy keeps the service worker out: %q", csp)
	}
	rec := do("POST", "/teilen")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("a share before the worker runs: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}
