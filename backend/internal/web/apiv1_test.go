package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestAPIv1IsMountedBehindTheLogin(t *testing.T) {
	saved := settings.Config
	t.Cleanup(func() { settings.Config = saved })
	settings.Config.Http.BaseURL = "/"
	settings.Config.Server.Sources = []*settings.Source{{Name: "Shares", Path: t.TempDir()}}

	router := http.NewServeMux()
	if err := mountAPIv1(router); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/fs/list?path=/", nil))
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("anonymous list: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/fs/list:") {
		t.Fatalf("contract: %d", rec.Code)
	}
}
