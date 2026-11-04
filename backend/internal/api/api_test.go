package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIFileIsCurrent(t *testing.T) {
	spec, err := NewRouter(v1.Deps{}, nil).OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	onDisk = bytes.ReplaceAll(onDisk, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(spec, onDisk) {
		t.Fatal("api/openapi.yaml is out of date, run: go run ./scripts/openapi")
	}
}

func TestOpenAPIReferencesResolve(t *testing.T) {
	spec, err := NewRouter(v1.Deps{}, nil).OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`\$ref: ['"]?#/([^'"\s]+)`).FindAllStringSubmatch(string(spec), -1)
	if len(refs) == 0 {
		t.Fatal("no references found")
	}
	for _, m := range refs {
		var node any = doc
		for _, part := range strings.Split(m[1], "/") {
			mm, ok := node.(map[string]any)
			if !ok {
				node = nil
				break
			}
			node = mm[part]
		}
		if node == nil {
			t.Errorf("$ref #/%s points nowhere", m[1])
		}
	}
}

func testAuth(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
	name := r.Header.Get("X-Test-User")
	if name == "" {
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	u := &users.User{}
	u.Username = name
	u.Permissions.Admin = name == "admin"
	return u, nil
}

func TestEveryOperationAnswersAsItsContractSays(t *testing.T) {
	r := NewRouter(v1.Deps{}, testAuth)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	param := regexp.MustCompile(`\{[^}]+\}`)
	for _, op := range r.Operations() {
		if op.Implemented() {
			continue
		}
		target := param.ReplaceAllString(op.Path, "x")
		call := func(user string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(strings.ToUpper(op.Method), target, nil)
			if user != "" {
				req.Header.Set("X-Test-User", user)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			return rec
		}

		want := http.StatusUnauthorized
		if op.Access == v1.AccessPublic {
			want = http.StatusNotImplemented
		}
		if rec := call(""); rec.Code != want {
			t.Errorf("%s without login: %d, want %d", op.Pattern(), rec.Code, want)
		}
		want = http.StatusNotImplemented
		if op.Access == v1.AccessAdmin {
			want = http.StatusForbidden
		}
		if rec := call("anna"); rec.Code != want {
			t.Errorf("%s as user: %d, want %d", op.Pattern(), rec.Code, want)
		}
		rec := call("admin")
		if rec.Code != http.StatusNotImplemented {
			t.Errorf("%s as admin: %d, want 501", op.Pattern(), rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s: content type %q", op.Pattern(), ct)
		}
		var p v1.Problem
		if err := json.NewDecoder(rec.Body).Decode(&p); err != nil || p.Status != http.StatusNotImplemented {
			t.Errorf("%s: problem body %+v, %v", op.Pattern(), p, err)
		}
	}
}

func TestUnknownEndpointIsAProblem(t *testing.T) {
	h, err := NewRouter(v1.Deps{}, testAuth).Handler()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestContractIsServed(t *testing.T) {
	h, err := NewRouter(v1.Deps{}, testAuth).Handler()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("got %d %.40q", rec.Code, rec.Body.String())
	}
}
