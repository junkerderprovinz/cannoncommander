//go:build !desktop

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

func TestTheContainerAnswersTheDesktopAppsOperationsWithDesktopOnly(t *testing.T) {
	r := NewRouter(v1.Deps{}, testAuth)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	desktopOnly := []string{"openwith", "shell", "places", "clipapi", "launch", "filemanager"}
	param := regexp.MustCompile(`\{[^}]+\}`)
	seen := 0
	for _, op := range r.Operations() {
		if !slices.Contains(desktopOnly, op.Tag) && op.Pattern() != "POST /terminal/native" {
			continue
		}
		seen++
		req := httptest.NewRequest(strings.ToUpper(op.Method), param.ReplaceAllString(op.Path, "x"), nil)
		req.Header.Set("X-Test-User", "admin")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var p v1.Problem
		_ = json.NewDecoder(rec.Body).Decode(&p)
		if rec.Code != http.StatusNotFound || p.Code != "desktopOnly" {
			t.Errorf("%s: %d %q", op.Pattern(), rec.Code, p.Code)
		}
	}
	if seen < len(desktopOnly)+1 {
		t.Fatalf("only %d operations of the desktop app are in the contract", seen)
	}
}
