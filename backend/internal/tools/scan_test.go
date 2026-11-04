package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex/fsindextest"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

type fixture struct {
	h     http.Handler
	share string
	m     *jobs.Manager
}

func put(t *testing.T, p, body string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if !at.IsZero() {
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	share := filepath.Join(t.TempDir(), "user")
	old, young := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	film := strings.Repeat("F", 200<<10)
	put(t, filepath.Join(share, "filme", "Sintel.mkv"), film, old)
	put(t, filepath.Join(share, "backup", "filme", "Sintel.mkv"), film, young)
	put(t, filepath.Join(share, "backup", "umbenannt.mkv"), film, old)
	// Same size and the same first and last 64 KiB, but a different
	// middle, which only the full comparison tells apart.
	fake := film[:100<<10] + "x" + film[100<<10+1:]
	put(t, filepath.Join(share, "backup", "falsch", "Sintel.mkv"), fake, old)
	put(t, filepath.Join(share, "fotos", "a.jpg"), "foto", old)
	put(t, filepath.Join(share, "fotos", "kopie", "a.jpg"), "foto", young)
	put(t, filepath.Join(share, "fotos", ".thumbs", "a.jpg"), "foto", old)
	put(t, filepath.Join(share, "leer1.txt"), "", old)
	put(t, filepath.Join(share, "leer2.txt"), "", old)
	put(t, filepath.Join(share, "docs", "plan.pdf"), strings.Repeat("p", 5000), young)
	put(t, filepath.Join(share, "docs", "notiz.txt"), "n", young)
	put(t, filepath.Join(share, ".cc-trash", "riesig.iso"), strings.Repeat("i", 900<<10), young)

	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true}
	people := map[string]*users.User{
		"jdp":  {Username: "jdp", BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {Username: "anna", BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
	}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	prev := jobOwner
	jobOwner = func(name string) (*users.User, error) {
		if u, ok := people[name]; ok {
			return u, nil
		}
		return nil, errors.New("no such user")
	}
	t.Cleanup(func() { jobOwner = prev })
	deps := v1.Deps{FS: fs, DB: testutil.MainDB(t), Hub: hub.New(hub.Options{})}
	m := jobs.For(deps)
	t.Cleanup(func() {
		m.Close()
		v1.Forget(deps, "jobs")
	})
	r := v1.NewRouter(deps, auth)
	fileops.Routes(r)
	Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{h: h, share: share, m: m}
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

func (f *fixture) usage(t *testing.T, user, query string) Usage {
	t.Helper()
	rec := f.do(t, user, http.MethodGet, "/tools/diskusage?"+query, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var u Usage
	if err := json.NewDecoder(rec.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestDiskUsageMeasuresTheFolder(t *testing.T) {
	f := newFixture(t)
	u := f.usage(t, "jdp", "path=/mnt/user")
	film := int64(200 << 10)
	if u.Size != 4*film+4+4+5001 || u.Files != 10 || u.Dirs != 7 {
		t.Errorf("totals = %+v", u.UsageTotals)
	}
	if len(u.Items) == 0 || u.Items[0].Name != "backup" || !u.Items[0].Dir || u.Items[0].Size != 3*film || u.Items[0].Files != 3 || u.Items[0].Dirs != 2 {
		t.Errorf("first item = %+v", u.Items[0])
	}
	if u.Types[0].Kind != "video" || u.Types[0].Files != 4 || u.Types[0].Exts[0] != "mkv" {
		t.Errorf("types = %+v", u.Types)
	}
	if len(u.Largest) != 10 || u.Largest[0].Size != film || u.Largest[len(u.Largest)-1].Size != 0 {
		t.Errorf("largest = %+v", u.Largest)
	}
	for _, it := range u.Items {
		if it.Name == ".cc-trash" || it.Name == "fotos" && it.Files != 2 {
			t.Errorf("item %+v", it)
		}
	}
	docs := f.usage(t, "jdp", "path=/mnt/user&kind=doc&largest=5")
	if len(docs.Largest) != 1 || docs.Largest[0].Name != "plan.pdf" || docs.Largest[0].At != "/mnt/user/docs" {
		t.Errorf("largest documents = %+v", docs.Largest)
	}
}

func TestDiskUsageKeepsToTheScope(t *testing.T) {
	f := newFixture(t)
	if rec := f.do(t, "anna", http.MethodGet, "/tools/diskusage?path=/mnt/user/docs", nil); rec.Code != http.StatusForbidden {
		t.Errorf("status %d", rec.Code)
	}
	u := f.usage(t, "anna", "path=/mnt/user")
	if u.Files != 2 {
		t.Errorf("anna sees %d files: %+v", u.Files, u.Items)
	}
	if rec := f.do(t, "jdp", http.MethodGet, "/tools/diskusage?path=/mnt/user&largest=-1", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("negative largest: status %d", rec.Code)
	}
}

func TestDiskUsageFromTheIndex(t *testing.T) {
	f := newFixture(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", f.share)
	u := f.usage(t, "jdp", "path=/mnt/user/fotos")
	if !u.Indexed || u.Files != 2 {
		t.Errorf("usage = %+v", u)
	}
}

func (f *fixture) duplicates(t *testing.T, p DuplicatesParams) Duplicates {
	t.Helper()
	rec := f.do(t, "jdp", http.MethodPost, "/tools/duplicates", p)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var j jobs.Job
	if err := json.NewDecoder(rec.Body).Decode(&j); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for j.State.Open() {
		if time.Now().After(deadline) {
			t.Fatal("job does not finish")
		}
		time.Sleep(10 * time.Millisecond)
		j, _ = f.m.Get(j.ID, true)
	}
	if j.State != jobs.Done {
		t.Fatalf("job %s: %s", j.State, j.Error)
	}
	var d Duplicates
	b, _ := json.Marshal(j.Result)
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func paths(g DupGroup) []string {
	out := make([]string, len(g.Copies))
	for i, c := range g.Copies {
		out[i] = c.Path
	}
	return out
}

func TestDuplicatesByNameCompareTheContent(t *testing.T) {
	f := newFixture(t)
	d := f.duplicates(t, DuplicatesParams{Path: "/mnt/user"})
	if len(d.Groups) != 2 {
		t.Fatalf("groups = %+v", d.Groups)
	}
	film := d.Groups[0]
	if got := paths(film); len(got) != 2 || got[0] != "/mnt/user/backup/filme/Sintel.mkv" || got[1] != "/mnt/user/filme/Sintel.mkv" {
		t.Errorf("film copies = %v", got)
	}
	if film.Wasted != 200<<10 || len(film.SHA256) != 64 {
		t.Errorf("film group = %+v", film)
	}
	if got := paths(d.Groups[1]); len(got) != 2 || got[0] != "/mnt/user/fotos/kopie/a.jpg" {
		t.Errorf("photo copies = %v", got)
	}
	if d.Wasted != 200<<10+4 || d.Files != 8 {
		t.Errorf("wasted %d, files %d", d.Wasted, d.Files)
	}
}

func TestDuplicatesBySizeFindRenamedCopies(t *testing.T) {
	f := newFixture(t)
	d := f.duplicates(t, DuplicatesParams{Path: "/mnt/user", By: "size", Hidden: true})
	if len(d.Groups) != 2 || len(d.Groups[0].Copies) != 3 || len(d.Groups[1].Copies) != 3 {
		t.Errorf("groups = %+v", d.Groups)
	}
}

func TestDuplicatesCompareFilesWhoseIndexedSizeIsTheirSpaceOnDisk(t *testing.T) {
	f := newFixture(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", f.share)
	// Unless a source asks for logical sizes, the scanner stores the blocks
	// a file takes, which is more than the file holds.
	if _, err := db.DB().Exec(`UPDATE index_items SET size = (size + 4095) / 4096 * 4096`); err != nil {
		t.Fatal(err)
	}
	d := f.duplicates(t, DuplicatesParams{Path: "/mnt/user"})
	if len(d.Groups) != 2 {
		t.Fatalf("groups = %+v", d.Groups)
	}
	photo := d.Groups[1]
	if photo.Name != "a.jpg" || photo.Size != 4 || photo.Wasted != 4 {
		t.Errorf("photo group = %+v", photo)
	}
}

func TestDuplicatesKeepToTheMinimumSizeOfTheFileItself(t *testing.T) {
	f := newFixture(t)
	db := fsindextest.New(t)
	fsindextest.Scan(t, db, "Shares", f.share)
	if _, err := db.DB().Exec(`UPDATE index_items SET size = (size + 4095) / 4096 * 4096`); err != nil {
		t.Fatal(err)
	}
	d := f.duplicates(t, DuplicatesParams{Path: "/mnt/user", MinSize: 100})
	if len(d.Groups) != 1 || d.Groups[0].Name != "Sintel.mkv" {
		t.Errorf("groups = %+v", d.Groups)
	}
}

func TestDuplicatesCheckTheirParams(t *testing.T) {
	f := newFixture(t)
	for _, p := range []DuplicatesParams{{Path: "/mnt/user", By: "farbe"}, {Path: "/mnt/user/fotos/a.jpg"}, {}, {Path: "/mnt/user", MinSize: -1}} {
		if rec := f.do(t, "jdp", http.MethodPost, "/tools/duplicates", p); rec.Code != http.StatusBadRequest {
			t.Errorf("%+v: status %d", p, rec.Code)
		}
	}
}
