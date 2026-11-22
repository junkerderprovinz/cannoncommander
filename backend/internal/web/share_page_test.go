package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestASharePageComesWithoutTheApp(t *testing.T) {
	saved := assetFs
	t.Cleanup(func() { assetFs = saved })
	assetFs = fstest.MapFS{
		"index.html": {Data: []byte("<html>app</html>")},
		"share.html": {Data: []byte(`<html><script type="module" src="/assets/share-abc.js"></script></html>`)},
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /s/{slug}", sharePageHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/s/k3x9qa-urlaub", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "share-abc.js") {
		t.Fatalf("page: %d %s", rec.Code, rec.Body)
	}
	h := rec.Header()
	if !strings.Contains(h.Get("Content-Security-Policy"), "script-src 'self';") || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Cache-Control") != "no-cache" {
		t.Fatalf("headers %v", h)
	}

	assetFs = fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/s/k3x9qa-urlaub", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a build without the page: %d", rec.Code)
	}
}
