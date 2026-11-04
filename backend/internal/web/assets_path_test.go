package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestIndexTemplatePath_DevMode(t *testing.T) {
	prev := settings.Env.IsDevMode
	settings.Env.IsDevMode = true
	t.Cleanup(func() { settings.Env.IsDevMode = prev })

	if got := indexTemplatePath(); got != "index.html" {
		t.Fatalf("indexTemplatePath() = %q, want index.html", got)
	}
}

func TestIndexTemplatePath_Production(t *testing.T) {
	prev := settings.Env.IsDevMode
	settings.Env.IsDevMode = false
	t.Cleanup(func() { settings.Env.IsDevMode = prev })

	if got := indexTemplatePath(); got != "public/index.html" {
		t.Fatalf("indexTemplatePath() = %q, want public/index.html", got)
	}
}

func TestIndexTemplatePath_ViteBuild(t *testing.T) {
	prevDev, prevFS := settings.Env.IsDevMode, fileutils.GetAssetFS()
	settings.Env.IsDevMode = false
	fileutils.InitAssetFS(fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}, true, false)
	t.Cleanup(func() {
		settings.Env.IsDevMode = prevDev
		fileutils.InitAssetFS(prevFS, true, false)
	})

	if got := indexTemplatePath(); got != "index.html" {
		t.Fatalf("indexTemplatePath() = %q, want index.html", got)
	}
}

func TestViteAssetsAreServedWithLongCaching(t *testing.T) {
	prevFS := assetFs
	assetFs = fstest.MapFS{"assets/index-abc123.js": {Data: []byte("console.log(1)")}}
	t.Cleanup(func() { assetFs = prevFS })

	rec := httptest.NewRecorder()
	viteAssetHandler(rec, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q", ct)
	}

	rec = httptest.NewRecorder()
	viteAssetHandler(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset: %d", rec.Code)
	}
}
