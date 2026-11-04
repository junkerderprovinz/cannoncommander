package fsapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

type fixture struct {
	h     http.Handler
	share string
	cache string
}

func write(t *testing.T, p string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T, index ...fsindex.Index) fixture {
	t.Helper()
	base := t.TempDir()
	share := filepath.Join(base, "mnt", "user")
	cache := filepath.Join(base, "mnt", "cache")
	boot := filepath.Join(base, "boot")
	write(t, filepath.Join(share, "filme", "Sintel (2010)", "Sintel (2010).mkv"), 2048)
	write(t, filepath.Join(share, "filme", "Sintel (2010)", "poster.jpg"), 100)
	write(t, filepath.Join(share, "fotos", "2025", "IMG_0001.jpg"), 10)
	write(t, filepath.Join(share, "downloads", ".partial-download"), 5)
	write(t, filepath.Join(share, "downloads", "setup-notes.md"), 4)
	if err := os.MkdirAll(filepath.Join(share, "musik", "Podcasts"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(boot, "config", "go"), 1)
	mtime := time.UnixMilli(1_700_000_000_000)
	if err := os.Chtimes(filepath.Join(share, "downloads", "setup-notes.md"), mtime, mtime); err != nil {
		t.Fatal(err)
	}

	fs, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share},
		ccfs.Source{Name: "USB-Stick", Mount: "/boot", Root: boot, ReadOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	people := map[string]*users.User{
		"jdp":  {BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}, {Path: boot, Scope: "/", Permissions: all}}},
		"anna": {BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
		"gast": {BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: users.SourceFilePermissions{View: true}}}},
	}
	for name, u := range people {
		u.Username = name
	}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	deps := v1.Deps{FS: fs}
	if len(index) > 0 {
		deps.Index = index[0]
	}
	r := v1.NewRouter(deps, auth)
	Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return fixture{h: h, share: share, cache: cache}
}

func (f fixture) get(t *testing.T, user, endpoint, p string, extra ...string) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{"path": {p}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	req := httptest.NewRequest(http.MethodGet, endpoint+"?"+q.Encode(), nil)
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func names(items []Item) []string {
	out := make([]string, len(items))
	for i, e := range items {
		out[i] = e.Name
	}
	return out
}

func TestListShowsTheVirtualRoot(t *testing.T) {
	f := newFixture(t)
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/"))
	if l.Volume != nil || len(l.Items) != 2 || l.Items[0].Name != "boot" || l.Items[1].Name != "mnt" {
		t.Fatalf("listing of / = %+v", l)
	}
	l = decode[Listing](t, f.get(t, "anna", "/fs/list", "/"))
	if len(l.Items) != 1 || l.Items[0].Name != "mnt" {
		t.Fatalf("anna sees %v at /", names(l.Items))
	}
}

func TestListReturnsItemsWithTheirFields(t *testing.T) {
	f := newFixture(t)
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/downloads"))
	if l.Path != "/mnt/user/downloads" {
		t.Fatalf("listing = %+v", l)
	}
	if l.Volume == nil || l.Volume.Path != "/mnt/user" || l.Volume.Label != "Shares" || l.Volume.Total == 0 {
		t.Errorf("volume = %+v", l.Volume)
	}
	if len(l.Items) != 2 {
		t.Fatalf("items = %v", names(l.Items))
	}
	hidden, notes := l.Items[0], l.Items[1]
	if hidden.Name != ".partial-download" || !hidden.Hidden || hidden.Size != 5 {
		t.Errorf("hidden item = %+v", hidden)
	}
	if notes.Name != "setup-notes.md" || notes.Dir || notes.Size != 4 || notes.MTime != "2023-11-14T22:13:20.000Z" || len(notes.Mode) != 4 {
		t.Errorf("notes item = %+v", notes)
	}

	l = decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user"))
	for _, e := range l.Items {
		if !e.Dir || e.Size != 0 {
			t.Errorf("folder item = %+v", e)
		}
	}
}

func TestListKeepsUsersInTheirScope(t *testing.T) {
	f := newFixture(t)
	l := decode[Listing](t, f.get(t, "anna", "/fs/list", "/mnt/user"))
	if got := names(l.Items); len(got) != 1 || got[0] != "fotos" {
		t.Fatalf("anna sees %v in /mnt/user", got)
	}
	if rec := f.get(t, "anna", "/fs/list", "/mnt/user/filme"); rec.Code != http.StatusForbidden {
		t.Fatalf("anna lists filme: %d", rec.Code)
	}
	if rec := f.get(t, "anna", "/fs/list", "/boot"); rec.Code != http.StatusForbidden {
		t.Fatalf("anna lists /boot: %d", rec.Code)
	}
}

func TestListRejectsBadInput(t *testing.T) {
	f := newFixture(t)
	cases := map[string]int{
		"":                                   http.StatusBadRequest,
		"relative":                           http.StatusBadRequest,
		"/mnt/user/../../etc":                http.StatusBadRequest,
		"/etc":                               http.StatusNotFound,
		"/mnt/user/nichts":                   http.StatusNotFound,
		"/mnt/user/downloads/setup-notes.md": http.StatusBadRequest,
		"/mnt/user/a.zip::/":                 http.StatusNotImplemented,
	}
	for p, want := range cases {
		rec := f.get(t, "jdp", "/fs/list", p)
		if rec.Code != want {
			t.Errorf("list %q: %d, want %d (%s)", p, rec.Code, want, rec.Body)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("list %q: content type %q", p, ct)
		}
	}
}

func TestListAnswersNotModified(t *testing.T) {
	f := newFixture(t)
	first := f.get(t, "jdp", "/fs/list", "/mnt/user/filme")
	tag := first.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/fs/list?path=/mnt/user/filme", nil)
	req.Header.Set("X-User", "jdp")
	req.Header.Set("If-None-Match", tag)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("second request: %d", rec.Code)
	}

	write(t, filepath.Join(f.share, "filme", "Neu.mkv"), 1)
	rec = httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("after a change: %d", rec.Code)
	}
}

func treeNames(tr Tree) []string {
	var out []string
	for _, it := range tr.Items {
		out = append(out, it.Name)
	}
	return out
}

func TestTreeListsSubfoldersOneLevelDeep(t *testing.T) {
	f := newFixture(t)
	root := decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/"))
	if root.Path != "/" || len(root.Items) != 2 || !root.Items[1].HasDirs || root.Items[1].Path != "/mnt" {
		t.Fatalf("root = %+v", root)
	}
	mnt := decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/mnt"))
	if len(mnt.Items) != 1 || mnt.Items[0].Label != "Shares" || mnt.Items[0].Total == 0 {
		t.Fatalf("mnt = %+v", mnt)
	}
	share := decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/mnt/user"))
	if got := treeNames(share); len(got) != 4 || got[0] != "downloads" || got[3] != "musik" {
		t.Fatalf("share = %v", got)
	}
	for _, it := range share.Items {
		if want := it.Name != "downloads"; it.HasDirs != want {
			t.Errorf("%s hasDirs = %v, want %v", it.Name, it.HasDirs, want)
		}
		if it.Label != "" {
			t.Errorf("%s has label %q", it.Name, it.Label)
		}
	}
}

func TestTreeShowsOnlyTheWayToTheScope(t *testing.T) {
	f := newFixture(t)
	tr := decode[Tree](t, f.get(t, "anna", "/fs/tree", "/mnt/user"))
	if len(tr.Items) != 1 || tr.Items[0].Name != "fotos" || !tr.Items[0].HasDirs {
		t.Fatalf("anna tree = %+v", tr)
	}
}
