package meta

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func router(t *testing.T) (http.Handler, string) {
	t.Helper()
	share := filepath.Join(t.TempDir(), "user")
	writeFile(t, filepath.Join(share, "fotos", "DSCF0001.jpg"), photoJPEG(t))
	writeFile(t, filepath.Join(share, "filme", "notes.txt"), []byte("eins\nzwei\n"))
	writeFile(t, filepath.Join(share, "docs", "angebot.docx"), docx(t))
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true}
	people := map[string]*users.User{
		"jdp":  {Username: "jdp", BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {Username: "anna", BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
		"gast": {Username: "gast", BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: users.SourceFilePermissions{View: true}}}},
	}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	r := v1.NewRouter(v1.Deps{FS: fs}, auth)
	r.Contract("fs", []byte(`tag: {name: fs}
paths:
  /fs/meta: {post: {operationId: fsMeta}}
  /fs/page: {get: {operationId: fsPage}}
`))
	s := newService(t)
	Routes(r, s)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return h, share
}

func call(t *testing.T, h http.Handler, user, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMetaAnswersPerAskedPath(t *testing.T) {
	h, _ := router(t)
	rec := call(t, h, "jdp", http.MethodPost, "/fs/meta", metaRequest{
		Paths:   []string{"/mnt/user/fotos/DSCF0001.jpg", "/mnt/user/filme/notes.txt", "/mnt/user/fotos", "/mnt/user/gibts/nicht.jpg"},
		Columns: []string{"camera", "lines", "name", "nonsense"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got metaResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Items["/mnt/user/fotos/DSCF0001.jpg"]["camera"] != "X100VI" {
		t.Errorf("photo = %v", got.Items["/mnt/user/fotos/DSCF0001.jpg"])
	}
	if got.Items["/mnt/user/filme/notes.txt"]["lines"] != float64(2) {
		t.Errorf("text = %v", got.Items["/mnt/user/filme/notes.txt"])
	}
	if len(got.Items) != 2 {
		t.Errorf("items = %v, want the photo and the text only", got.Items)
	}
}

func TestMetaLeavesOutWhatTheUserMayNotSee(t *testing.T) {
	h, _ := router(t)
	rec := call(t, h, "anna", http.MethodPost, "/fs/meta", metaRequest{
		Paths:   []string{"/mnt/user/fotos/DSCF0001.jpg", "/mnt/user/filme/notes.txt"},
		Columns: []string{"camera", "lines"},
	})
	var got metaResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Items["/mnt/user/filme/notes.txt"]; ok || len(got.Items) != 1 {
		t.Errorf("anna sees %v", got.Items)
	}
}

func TestMetaRejectsHugeRequests(t *testing.T) {
	h, _ := router(t)
	rec := call(t, h, "jdp", http.MethodPost, "/fs/meta", metaRequest{Paths: make([]string, maxMetaPaths+1), Columns: []string{"camera"}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status %d", rec.Code)
	}
}

func TestPageChecksItsRequest(t *testing.T) {
	h, _ := router(t)
	cases := []struct {
		user, target string
		want         int
	}{
		{"jdp", "/fs/page?path=/mnt/user/docs/angebot.docx&width=10", http.StatusBadRequest},
		{"jdp", "/fs/page?path=/mnt/user/docs/angebot.docx&page=0", http.StatusBadRequest},
		{"jdp", "/fs/page?path=/mnt/user/filme/notes.txt", http.StatusUnsupportedMediaType},
		{"gast", "/fs/page?path=/mnt/user/docs/angebot.docx", http.StatusForbidden},
		{"jdp", "/fs/page?path=/mnt/user/docs/fehlt.pdf", http.StatusNotFound},
	}
	for _, c := range cases {
		if rec := call(t, h, c.user, http.MethodGet, c.target, nil); rec.Code != c.want {
			t.Errorf("%s as %s: status %d, want %d: %s", c.target, c.user, rec.Code, c.want, rec.Body)
		}
	}
}

func TestPageDrawsAPDF(t *testing.T) {
	haveTools(t, "mutool")
	h, share := router(t)
	writeFile(t, filepath.Join(share, "docs", "steuer.pdf"), twoPagePDF())
	rec := call(t, h, "jdp", http.MethodGet, "/fs/page?path=/mnt/user/docs/steuer.pdf&page=1&width=200&v=1", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status %d %s: %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	etag := rec.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/fs/page?path=/mnt/user/docs/steuer.pdf&page=1&width=200&v=1", nil)
	req.Header.Set("X-User", "jdp")
	req.Header.Set("If-None-Match", etag)
	again := httptest.NewRecorder()
	h.ServeHTTP(again, req)
	if again.Code != http.StatusNotModified {
		t.Errorf("second ask: status %d", again.Code)
	}
	if rec := call(t, h, "jdp", http.MethodGet, "/fs/page?path=/mnt/user/docs/steuer.pdf&page=9", nil); rec.Code != http.StatusNotFound {
		t.Errorf("page 9: status %d", rec.Code)
	}
}
