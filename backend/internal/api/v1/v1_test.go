package v1

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

const sample = `
tag:
  name: demo
paths:
  /demo/{id}:
    get:
      operationId: demoGet
      summary: Demo
    post:
      operationId: demoPost
      x-access: public
`

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	fn()
}

func TestContractRejectsMistakes(t *testing.T) {
	cases := map[string]string{
		"no tag":          "paths: {/x: {get: {operationId: a}}}",
		"no operationId":  "tag: {name: t}\npaths: {/x: {get: {summary: a}}}",
		"relative path":   "tag: {name: t}\npaths: {x: {get: {operationId: a}}}",
		"unknown access":  "tag: {name: t}\npaths: {/x: {get: {operationId: a, x-access: root}}}",
		"not yaml at all": "tag: [",
	}
	for name, spec := range cases {
		mustPanic(t, name, func() { NewRouter(Deps{}, nil).Contract(name, []byte(spec)) })
	}
	r := NewRouter(Deps{}, nil)
	r.Contract("a", []byte(sample))
	mustPanic(t, "same operation twice", func() { r.Contract("b", []byte(sample)) })
	mustPanic(t, "unknown handler", func() { r.Handle("GET /nowhere", nil) })
	r.Handle("GET /demo/{id}", func(http.ResponseWriter, *http.Request) error { return nil })
	mustPanic(t, "bound twice", func() {
		r.Handle("GET /demo/{id}", func(http.ResponseWriter, *http.Request) error { return nil })
	})
}

func TestMergeFillsInWhatContractsLeaveOut(t *testing.T) {
	r := NewRouter(Deps{}, nil)
	r.Contract("demo", []byte(sample))
	spec, err := r.OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	s := string(spec)
	for _, want := range []string{"tags: [demo]", "name: id", "in: path", "#/components/responses/NotImplemented", "security: []"} {
		if !strings.Contains(s, want) {
			t.Errorf("merged spec lacks %q", want)
		}
	}
}

func TestHandlerChecksAccessAndBindsUser(t *testing.T) {
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if r.Header.Get("X-User") == "" {
			return nil, Errorf(http.StatusUnauthorized, "login required")
		}
		u := &users.User{}
		u.Username = r.Header.Get("X-User")
		return u, nil
	}
	r := NewRouter(Deps{}, auth)
	r.Contract("demo", []byte(sample))
	r.Handle("GET /demo/{id}", func(w http.ResponseWriter, req *http.Request) error {
		return WriteJSON(w, req, http.StatusOK, map[string]string{"user": UserFrom(req).Username, "id": req.PathValue("id")})
	})
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/demo/7", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/demo/7", nil)
	req.Header.Set("X-User", "anna")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"user":"anna"`) || !strings.Contains(rec.Body.String(), `"id":"7"`) {
		t.Fatalf("logged in: %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/demo/7", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("public stub: %d", rec.Code)
	}
}

func TestWriteJSONAnswersNotModifiedForAMatchingETag(t *testing.T) {
	body := map[string]int{"n": 1}
	rec := httptest.NewRecorder()
	if err := WriteJSON(rec, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusOK, body); err != nil {
		t.Fatal(err)
	}
	tag := rec.Header().Get("ETag")
	if !strings.HasPrefix(tag, `W/"`) {
		t.Fatalf("ETag = %q", tag)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `"other", `+tag)
	rec = httptest.NewRecorder()
	if err := WriteJSON(rec, req, http.StatusOK, body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("got %d with %d bytes", rec.Code, rec.Body.Len())
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", tag)
	rec = httptest.NewRecorder()
	if err := WriteJSON(rec, req, http.StatusOK, map[string]int{"n": 2}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("changed body got %d", rec.Code)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	var v struct {
		Path string `json:"path"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"path":"/mnt","paht":"/x"}`))
	err := DecodeJSON(req, &v)
	var p *Problem
	if !errors.As(err, &p) || p.Status != http.StatusBadRequest {
		t.Fatalf("err = %v", err)
	}
}

func TestProblemForMapsKnownErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ccfs.ErrBadPath, http.StatusBadRequest},
		{fmt.Errorf("open: %w", fs.ErrNotExist), http.StatusNotFound},
		{ccfs.ErrForbidden, http.StatusForbidden},
		{ccfs.ErrEscape, http.StatusForbidden},
		{ccfs.ErrReadOnly, http.StatusForbidden},
		{Errorf(http.StatusConflict, "exists"), http.StatusConflict},
		{errors.New("disk on fire"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		p := ProblemFor(c.err)
		if p.Status != c.want {
			t.Errorf("ProblemFor(%v) = %d, want %d", c.err, p.Status, c.want)
		}
		if c.want == http.StatusInternalServerError && p.Detail != "" {
			t.Errorf("internal error leaked detail %q", p.Detail)
		}
	}
}

func TestAPermissionErrorOfAnySystemIsDenied(t *testing.T) {
	err := &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}
	if code, _ := CodeOf(err); code != "denied" {
		t.Fatalf("got %q", code)
	}
}

func TestADesktopContractAnswersDesktopOnly(t *testing.T) {
	r := NewRouter(Deps{}, func(http.ResponseWriter, *http.Request) (*users.User, error) { return &users.User{}, nil })
	r.DesktopContract("demo", []byte(sample))
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/demo/7", nil))
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"desktopOnly"`) {
			t.Errorf("%s: %d %s", method, rec.Code, rec.Body)
		}
	}
	spec, err := r.OpenAPI()
	if err != nil || !strings.Contains(string(spec), "operationId: demoPost") {
		t.Fatalf("the contract is not served: %v", err)
	}
}
