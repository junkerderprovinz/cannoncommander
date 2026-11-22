package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

)

func TestAPIv1IsMountedBehindTheLogin(t *testing.T) {
	router := newServer(t, withAdminPassword(t))

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
