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
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.HasPrefix(csp, "script-src 'self' https://office.tower.lan;") {
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
