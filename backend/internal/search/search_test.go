package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex/fsindextest"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type fixture struct {
	h     http.Handler
	share string
	deps  v1.Deps
	m     *jobs.Manager
	users map[string]*users.User
}

func put(t *testing.T, p string, body string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		at := time.Now().Add(-age)
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	share := filepath.Join(t.TempDir(), "user")
	put(t, filepath.Join(share, "filme", "Sintel (2010)", "Sintel (2010).mkv"), strings.Repeat("x", 3000), 0)
	put(t, filepath.Join(share, "filme", "Sintel (2010)", "poster.jpg"), "jpg", 0)
	put(t, filepath.Join(share, "filme", "Serie", "Serie.S01E02.mkv"), "mkv", 0)
	put(t, filepath.Join(share, "filme", ".versteckt", "geheim.mkv"), "mkv", 0)
	put(t, filepath.Join(share, "fotos", "2025", "IMG_0002.jpg"), "a", 0)
	put(t, filepath.Join(share, "fotos", "2025", "IMG_10.jpg"), "b", 0)
	put(t, filepath.Join(share, "fotos", "2025", "IMG_9.jpg"), "c", 0)
	put(t, filepath.Join(share, "docs", "notizen.txt"), "Router 192.168.10.1\nDer Parity-Check läuft sonntags\nEnde\n", 0)
	put(t, filepath.Join(share, "docs", "bericht.md"), "# Bericht\n", 800*24*time.Hour)
	put(t, filepath.Join(share, "docs", "blob.bin"), "\x00\x01parity", 0)
	put(t, filepath.Join(share, ".cc-trash", "weg.txt"), "parity", 0)
	put(t, filepath.Join(share, "docs", ".notiz.txt.cc-part"), "parity", 0)
	// The text sits across the line between the first and the second
	// megabyte the search reads.
	big := strings.Repeat("a", 1<<20-3) + "PARITY\n"
	put(t, filepath.Join(share, "docs", "gross.log"), big, 0)

	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	people := map[string]*users.User{
		"jdp":  {Username: "jdp", BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {Username: "anna", BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
	}
	people["jdp"].Permissions.Admin = true
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	old := userByName
	userByName = func(name string) (*users.User, error) {
		if u, ok := people[name]; ok {
			return u, nil
		}
		return nil, errors.New("no such user")
	}
	t.Cleanup(func() { userByName = old })

	deps := v1.Deps{FS: fs, DB: testutil.MainDB(t), Hub: hub.New(hub.Options{})}
	m := jobs.For(deps)
	t.Cleanup(func() {
		m.Close()
		v1.Forget(deps, "jobs")
	})
	r := v1.NewRouter(deps, auth)
	Routes(r)
	r.Contract("system", []byte(`tag: {name: system}
paths:
  /admin/index:
    get: {operationId: indexState, x-access: admin}
    post: {operationId: indexRebuild, x-access: admin}
`))
	IndexRoutes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{h: h, share: share, deps: deps, m: m, users: people}
}

func (f *fixture) do(t *testing.T, user, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) search(t *testing.T, user string, args ...string) Result {
	t.Helper()
	q := url.Values{}
	for i := 0; i+1 < len(args); i += 2 {
		q.Set(args[i], args[i+1])
	}
	rec := f.do(t, user, http.MethodGet, "/search?"+q.Encode(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var res Result
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	return res
}

func names(res Result) []string {
	out := make([]string, len(res.Items))
	for i, h := range res.Items {
		out[i] = h.Name
	}
	return out
}

func TestSearchFindsByNameBelowTheFolder(t *testing.T) {
	f := newFixture(t)
	res := f.search(t, "jdp", "path", "/mnt/user", "q", "*.mkv")
	if got := names(res); !slices.Equal(got, []string{"Serie.S01E02.mkv", "Sintel (2010).mkv"}) {
		t.Errorf("hits = %v", got)
	}
	if res.Items[1].At != "/mnt/user/filme/Sintel (2010)" || res.Items[1].Size != 3000 || res.Total != 2 || res.Indexed {
		t.Errorf("hit = %+v, result = %+v", res.Items[1], res)
	}
	hidden := f.search(t, "jdp", "path", "/mnt/user", "q", "*.mkv", "hidden", "true")
	if !slices.Contains(names(hidden), "geheim.mkv") {
		t.Errorf("hidden search = %v", names(hidden))
	}
	if got := names(f.search(t, "jdp", "path", "/mnt/user", "q", "parity", "hidden", "true")); len(got) != 0 {
		t.Errorf("the trash and part files show up: %v", got)
	}
}

func TestSearchPutsFoldersFirstAndNumbersInOrder(t *testing.T) {
	f := newFixture(t)
	got := names(f.search(t, "jdp", "path", "/mnt/user", "q", "img 2025"))
	if !slices.Equal(got, []string{"2025", "IMG_0002.jpg", "IMG_9.jpg", "IMG_10.jpg"}) {
		t.Errorf("order = %v", got)
	}
	limited := f.search(t, "jdp", "path", "/mnt/user", "q", "img", "limit", "2")
	if !slices.Equal(names(limited), []string{"IMG_0002.jpg", "IMG_9.jpg"}) || limited.Total != 3 || !limited.More {
		t.Errorf("limited = %+v", limited)
	}
}

func TestSearchKeepsToTheScope(t *testing.T) {
	f := newFixture(t)
	got := names(f.search(t, "anna", "path", "/mnt/user", "q", "*"))
	for _, n := range got {
		if strings.HasSuffix(n, ".mkv") || n == "docs" || n == "notizen.txt" {
			t.Fatalf("anna finds %s: %v", n, got)
		}
	}
	if !slices.Contains(got, "IMG_9.jpg") {
		t.Errorf("anna misses her photos: %v", got)
	}
}

func TestSearchWindowFilters(t *testing.T) {
	f := newFixture(t)
	if got := names(f.search(t, "jdp", "path", "/mnt/user/docs", "type", "doc")); !slices.Equal(got, []string{"bericht.md", "gross.log", "notizen.txt"}) {
		t.Errorf("documents = %v", got)
	}
	if got := names(f.search(t, "jdp", "path", "/mnt/user", "q", "*.mkv", "min", "2 KB")); !slices.Equal(got, []string{"Sintel (2010).mkv"}) {
		t.Errorf("at least 2 KB = %v", got)
	}
	if got := names(f.search(t, "jdp", "path", "/mnt/user/docs", "when", "old")); !slices.Equal(got, []string{"bericht.md"}) {
		t.Errorf("older than a year = %v", got)
	}
	if got := names(f.search(t, "jdp", "path", "/mnt/user/filme", "type", "dir")); !slices.Equal(got, []string{"Serie", "Sintel (2010)"}) {
		t.Errorf("folders = %v", got)
	}
}

func TestSearchReadsMetadataForFields(t *testing.T) {
	f := newFixture(t)
	res := f.search(t, "jdp", "path", "/mnt/user/docs", "q", "zeilen:>=3")
	if !slices.Equal(names(res), []string{"notizen.txt"}) {
		t.Errorf("hits = %v", names(res))
	}
	if got := names(f.search(t, "jdp", "path", "/mnt/user/filme", "q", "staffel:1 folge:2")); !slices.Equal(got, []string{"Serie.S01E02.mkv"}) {
		t.Errorf("episodes = %v", got)
	}
}

func TestSearchNamesUnknownFields(t *testing.T) {
	f := newFixture(t)
	res := f.search(t, "jdp", "path", "/mnt/user", "q", "*.mkv quatsch:1")
	if len(res.Items) != 0 || !slices.Equal(res.Unknown, []string{"quatsch"}) {
		t.Errorf("result = %+v", res)
	}
}

func TestSearchRejectsBadCriteria(t *testing.T) {
	f := newFixture(t)
	for _, target := range []string{
		"/search", "/search?path=/mnt/user&type=musik", "/search?path=/mnt/user&min=viel", "/search?path=/mnt/user&when=gestern",
		"/search?path=/mnt/user/docs/notizen.txt", "/search?path=/mnt/user&text=parity", "/search?path=/mnt/user&limit=0",
	} {
		if rec := f.do(t, "jdp", http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d", target, rec.Code)
		}
	}
}

func TestSearchReadsTheIndex(t *testing.T) {
	f := newFixture(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", f.share)
	put(t, filepath.Join(f.share, "filme", "neu.mkv"), "x", 0)
	res := f.search(t, "jdp", "path", "/mnt/user", "q", "*.mkv")
	if !res.Indexed || slices.Contains(names(res), "neu.mkv") || len(res.Items) != 2 {
		t.Errorf("result = %+v", res)
	}
}

func waitFor(t *testing.T, m *jobs.Manager, id string) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		j, _ := m.Get(id, true)
		if !j.State.Open() {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("job still %s", j.State)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestContentSearchFindsTextInFiles(t *testing.T) {
	f := newFixture(t)
	c, _, _ := f.deps.Hub.Connect("client", "jdp", 0)
	defer f.deps.Hub.Disconnect(c)
	raw, _ := json.Marshal(ContentParams{Params: Params{Path: "/mnt/user"}, Text: "parity"})
	plan, err := prepareContent(context.Background(), f.deps, f.users["jdp"], raw)
	if err != nil {
		t.Fatal(err)
	}
	j, err := f.m.Submit("jdp", KindContent, plan, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	j = waitFor(t, f.m, j.ID)
	if j.State != jobs.Done {
		t.Fatalf("job %s: %s", j.State, j.Error)
	}
	var res ContentResult
	b, _ := json.Marshal(j.Result)
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, h := range res.Items {
		got[h.Name] = h.Line
	}
	if len(got) != 2 || got["notizen.txt"] != 2 || got["gross.log"] != 1 {
		t.Errorf("hits = %v", got)
	}
	var seen int
	for {
		select {
		case e := <-c.Events:
			if e.Type != EventHits {
				continue
			}
			var ev hitsEvent
			if err := json.Unmarshal(e.Data, &ev); err != nil {
				t.Fatal(err)
			}
			seen += len(ev.Items)
			if ev.Done {
				if seen != 2 || ev.Total != 2 {
					t.Errorf("events carried %d hits, total %d", seen, ev.Total)
				}
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no final search.hits event")
		}
	}
}

func TestContentSearchNeedsText(t *testing.T) {
	f := newFixture(t)
	for _, p := range []ContentParams{{Params: Params{Path: "/mnt/user"}}, {Params: Params{Path: "/mnt/user", Q: "x:1"}, Text: "a"}} {
		raw, _ := json.Marshal(p)
		if _, err := prepareContent(context.Background(), f.deps, f.users["jdp"], raw); err == nil {
			t.Errorf("%+v was accepted", p)
		}
	}
}

func TestFindInSeesTextAcrossChunks(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	put(t, p, strings.Repeat("ü\n", 1<<19)+"Größe", 0)
	line, ok, err := findIn(context.Background(), p, []byte("größe"))
	if err != nil || !ok || line != 1<<19+1 {
		t.Errorf("line %d, found %v, err %v", line, ok, err)
	}
}

type fakeIndex struct {
	passes   uint64
	requests int
	err      error
}

func (f *fakeIndex) RequestFullScan() error {
	f.requests++
	if f.err == nil {
		f.passes++
	}
	return f.err
}
func (f *fakeIndex) FullPasses() uint64              { return f.passes }
func (f *fakeIndex) ActiveScan() string              { return "" }
func (f *fakeIndex) GetNumDirs() uint64              { return 4 }
func (f *fakeIndex) GetNumFiles() uint64             { return 9 }
func (f *fakeIndex) GetStatus() indexing.IndexStatus { return indexing.READY }
func (f *fakeIndex) GetLastIndexed() time.Time       { return time.Unix(1_700_000_000, 0) }
func (f *fakeIndex) GetQuickScanTime() int           { return 1 }
func (f *fakeIndex) GetFullScanTime() int            { return 3 }

func TestAdminRebuildsTheIndexAsAJob(t *testing.T) {
	f := newFixture(t)
	ix := &fakeIndex{}
	old := indexOf
	indexOf = func(name string) sourceIndex {
		if name == "Shares" {
			return ix
		}
		return nil
	}
	t.Cleanup(func() { indexOf = old })

	if rec := f.do(t, "anna", http.MethodPost, "/admin/index", nil); rec.Code != http.StatusForbidden {
		t.Errorf("anna: status %d", rec.Code)
	}
	if rec := f.do(t, "jdp", http.MethodPost, "/admin/index", ReindexParams{Source: "Fehlt"}); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown source: status %d", rec.Code)
	}
	rec := f.do(t, "jdp", http.MethodPost, "/admin/index", ReindexParams{Source: "Shares"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var j jobs.Job
	if err := json.NewDecoder(rec.Body).Decode(&j); err != nil {
		t.Fatal(err)
	}
	if j = waitFor(t, f.m, j.ID); j.State != jobs.Done || ix.requests != 1 {
		t.Errorf("job %s (%s), %d requests", j.State, j.Error, ix.requests)
	}
	rec = f.do(t, "jdp", http.MethodGet, "/admin/index", nil)
	var st indexStates
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if len(st.Sources) != 1 || st.Sources[0].Status != "ready" || st.Sources[0].Files != 9 || st.Sources[0].Job != "" {
		t.Errorf("state = %+v", st)
	}
}
