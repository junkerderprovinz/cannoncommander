package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

func setTestSourceReadOnly(sourceName string, readOnly bool) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return
	}
	idx.Config.ReadOnly = readOnly
}

func TestDenyIfSourceReadOnly(t *testing.T) {
	t.Parallel()
	indexing.SetTestIndex("ro-test", t.TempDir())
	t.Cleanup(func() { indexing.ClearTestIndices() })

	if status, err := denyIfSourceReadOnly("ro-test"); status != 0 || err != nil {
		t.Fatalf("writable source: status=%d err=%v", status, err)
	}

	setTestSourceReadOnly("ro-test", true)
	status, err := denyIfSourceReadOnly("ro-test")
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("read-only source: status=%d err=%v", status, err)
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error = %v", err)
	}
}

func webDAVReadOnlyTestUser(source1Path string) *users.User {
	user := &users.User{
		ID:           1,
		FrontendUser: users.FrontendUser{Username: "writer"},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
		BackendSourcePermissions: webDAVPermsForPaths(true, true, true, true, source1Path),
		Version:                  users.SourcePermissionsMigrationVersion,
	}
	applyBackendSourcePerms(user, user.BackendSourcePermissions)
	return user
}

func callWebDAVHandler(t *testing.T, user *users.User, method, path string, body io.Reader, extra func(*http.Request)) (int, int) {
	t.Helper()
	req := httptest.NewRequest(method, "/dav/source1"+path, body)
	req.SetPathValue("source", "source1")
	req.SetPathValue("path", path)
	if extra != nil {
		extra(req)
	}
	w := httptest.NewRecorder()
	status, err := webDAVHandler(w, req, &requestContext{User: user})
	if status != 0 && status != http.StatusOK {
		if w.Code == 0 || w.Code == http.StatusOK {
			w.Code = status
		}
	}
	if err != nil && status == 0 {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return w.Code, status
}

func TestWebDAVReadOnlySourceEnforcement(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	user := webDAVReadOnlyTestUser(source1Path)

	t.Run("read-only", func(t *testing.T) {
		setTestSourceReadOnly("source1", true)
		t.Cleanup(func() { setTestSourceReadOnly("source1", false) })

		writeCases := []struct {
			method string
			path   string
			setup  func(*http.Request)
		}{
			{method: "MKCOL", path: "/public/rodir"},
			{method: http.MethodPut, path: "/public/ro-new.txt"},
			{method: http.MethodDelete, path: "/public/readme.txt"},
			{
				method: "COPY",
				path:   "/public/readme.txt",
				setup: func(r *http.Request) {
					r.Header.Set("Destination", "/dav/source1/public/readme-copy.txt")
				},
			},
			{
				method: "MOVE",
				path:   "/public/readme.txt",
				setup: func(r *http.Request) {
					r.Header.Set("Destination", "/dav/source1/public/readme-moved.txt")
				},
			},
		}
		for _, tc := range writeCases {
			t.Run(tc.method, func(t *testing.T) {
				var body io.Reader
				if tc.method == http.MethodPut {
					body = strings.NewReader("data")
				}
				code, _ := callWebDAVHandler(t, user, tc.method, tc.path, body, tc.setup)
				if code != http.StatusForbidden {
					t.Fatalf("expected 403, got %d", code)
				}
			})
		}

		for _, tc := range []struct {
			method string
			path   string
		}{
			{method: "PROPFIND", path: "/public"},
			{method: http.MethodGet, path: "/public/readme.txt"},
		} {
			t.Run(tc.method+"_allowed", func(t *testing.T) {
				code, _ := callWebDAVHandler(t, user, tc.method, tc.path, nil, nil)
				if code == http.StatusForbidden {
					t.Fatalf("%s should not be forbidden on read-only source", tc.method)
				}
			})
		}
	})

	t.Run("writable", func(t *testing.T) {
		setTestSourceReadOnly("source1", false)
		code, _ := callWebDAVHandler(t, user, http.MethodPut, "/public/writable-new.txt", strings.NewReader("ok"), nil)
		if code != http.StatusCreated {
			t.Fatalf("PUT on writable source: expected 201, got %d", code)
		}
	})
}
