package system

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

type fixture struct {
	t       *testing.T
	h       http.Handler
	svc     *service
	share   string
	applied []App
	tested  []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	share := filepath.Join(base, "user")
	for _, d := range []string{"appdata/cannoncommander/exports", "filme"} {
		if err := os.MkdirAll(filepath.Join(share, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(base, "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	people := map[string]*users.User{
		"jdp":  {ID: 1, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {ID: 2, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
	}
	for name, u := range people {
		u.Username = name
	}
	people["jdp"].Permissions.Admin = true
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}

	f := &fixture{t: t, share: share}
	deps := v1.Deps{FS: fs, DB: db}
	running := []*settings.Source{{Name: "Shares", Path: share}}
	f.svc = &service{
		deps:        deps,
		store:       storeFor(deps),
		apply:       func(a App) { f.applied = append(f.applied, a) },
		running:     func() []*settings.Source { return running },
		sourcesFile: func() string { return filepath.Join(base, settings.SourcesFileName) },
		test: func(_ context.Context, kind, url, secret string) IntegrationResult {
			f.tested = append(f.tested, kind+" "+url+" "+secret)
			return IntegrationResult{Kind: kind, OK: true, Reachable: true, Token: TokenOK}
		},
	}
	sections.mu.Lock()
	saved := sections.list
	sections.list = nil
	sections.mu.Unlock()
	t.Cleanup(func() {
		sections.mu.Lock()
		sections.list = saved
		sections.mu.Unlock()
	})
	f.svc.ownSections()

	r := v1.NewRouter(deps, auth)
	r.Contract("system", contract)
	bind(r, f.svc)
	f.h, err = r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) call(who, method, target, body string, out any) int {
	f.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-User", who)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if out != nil && rec.Code < 300 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			f.t.Fatalf("%s %s: %v in %s", method, target, err, rec.Body)
		}
	}
	return rec.Code
}

func TestAppSettingsSaveFieldByFieldAndHideSecrets(t *testing.T) {
	f := newFixture(t)
	var doc appDoc
	if code := f.call("jdp", "GET", "/admin/app", "", &doc); code != 200 {
		t.Fatalf("get: %d", code)
	}
	if doc.Office.Kind != OfficeNone || doc.LogLevel != "info" || doc.Remotes.Cache != CacheWrites || doc.Remotes.CacheSize != 50 || !doc.Remotes.Automount {
		t.Fatalf("defaults %+v", doc.App)
	}

	body := `{"office":{"kind":"onlyoffice","url":"https://office.tower.lan/","secret":"geheim"},"arrowloop":{"url":"http://arrowloop.lan:8080","token":"t0k"}}`
	if code := f.call("jdp", "PUT", "/admin/app", body, &doc); code != 200 {
		t.Fatalf("put: %d", code)
	}
	if doc.Office.Secret != "" || !doc.Office.SecretSet || doc.ArrowLoop.Token != "" || !doc.ArrowLoop.TokenSet {
		t.Fatalf("secrets leak or are not marked: %+v", doc.App)
	}
	if doc.Office.URL != "https://office.tower.lan" {
		t.Fatalf("url %q", doc.Office.URL)
	}
	if last := f.applied[len(f.applied)-1]; last.Office.Secret != "geheim" {
		t.Fatalf("applied %+v", last.Office)
	}

	// One switch at a time, and what the page read goes back unchanged.
	if code := f.call("jdp", "PUT", "/admin/app", `{"logLevel":"debug","restartNeeded":false}`, &doc); code != 200 {
		t.Fatalf("put level: %d", code)
	}
	got, err := f.svc.store.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.LogLevel != "debug" || got.Office.Secret != "geheim" || got.ArrowLoop.Token != "t0k" {
		t.Fatalf("after one switch %+v", got)
	}

	if code := f.call("jdp", "PUT", "/admin/app", `{"office":{"secret":""}}`, &doc); code != 200 || doc.Office.SecretSet {
		t.Fatalf("clearing the secret: %d %+v", code, doc.Office)
	}

	for _, bad := range []string{
		`{"logLevel":"loud"}`,
		`{"office":{"kind":"wordcraft"}}`,
		`{"office":{"kind":"collabora","url":""}}`,
		`{"office":{"url":"ftp://office"}}`,
		`{"remotes":{"cache":"sometimes"}}`,
		`{"remotes":{"cacheSize":0}}`,
		`{"colour":"red"}`,
	} {
		if code := f.call("jdp", "PUT", "/admin/app", bad, nil); code != 400 {
			t.Errorf("%s: %d, want 400", bad, code)
		}
	}
	if code := f.call("anna", "GET", "/admin/app", "", nil); code != 403 {
		t.Fatalf("user reads app settings: %d", code)
	}
}

func TestArrowLoopPathsAreReplacedAsAWhole(t *testing.T) {
	f := newFixture(t)
	put := func(body string, want int) appDoc {
		t.Helper()
		var doc appDoc
		if code := f.call("jdp", "PUT", "/admin/app", body, &doc); code != want {
			t.Fatalf("%s: %d, want %d", body, code, want)
		}
		return doc
	}
	put(`{"arrowloop":{"url":"http://al.lan","paths":{"/mnt/user":"/data","/mnt/cache":"/cache"}}}`, 200)
	if doc := put(`{"arrowloop":{"paths":{"/mnt/user":"/media"}}}`, 200); len(doc.ArrowLoop.Paths) != 1 || doc.ArrowLoop.Paths["/mnt/user"] != "/media" {
		t.Fatalf("paths after replacing %v", doc.ArrowLoop.Paths)
	}
	if doc := put(`{"logLevel":"debug"}`, 200); doc.ArrowLoop.Paths["/mnt/user"] != "/media" || doc.ArrowLoop.URL != "http://al.lan" {
		t.Fatalf("paths left out of the body %+v", doc.ArrowLoop)
	}
	put(`{"arrowloop":{"paths":{"/mnt/user":"data"}}}`, 400)
	got, err := f.svc.store.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ArrowLoop.Paths) != 1 || got.ArrowLoop.Paths["/mnt/user"] != "/media" {
		t.Fatalf("a refused save changed the paths to %v", got.ArrowLoop.Paths)
	}
	if doc := put(`{"arrowloop":{"paths":{}}}`, 200); len(doc.ArrowLoop.Paths) != 0 {
		t.Fatalf("paths after emptying %v", doc.ArrowLoop.Paths)
	}
}

func TestSettingsSurviveARestart(t *testing.T) {
	f := newFixture(t)
	if code := f.call("jdp", "PUT", "/admin/app", `{"previews":{"thumbs":false},"remotes":{"cacheSize":200}}`, nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	a, saved, err := readApp(context.Background(), f.svc.store.db)
	if err != nil || !saved {
		t.Fatalf("read: %v %v", saved, err)
	}
	if a.Previews.Thumbs || a.Remotes.CacheSize != 200 || a.Remotes.Cache != CacheWrites {
		t.Fatalf("read back %+v", a)
	}
}

func TestSourcesAreCheckedAndSavedForTheNextStart(t *testing.T) {
	f := newFixture(t)
	var out Sources
	if code := f.call("jdp", "GET", "/admin/sources", "", &out); code != 200 {
		t.Fatalf("get: %d", code)
	}
	if len(out.Sources) != 1 || out.Sources[0].Name != "Shares" || out.RestartNeeded || !out.Sources[0].Indexed {
		t.Fatalf("running sources %+v", out)
	}

	filme := filepath.Join(f.share, "filme")
	for _, bad := range []string{
		`{"sources":[]}`,
		`{"sources":[{"name":"","path":"` + js(f.share) + `"}]}`,
		`{"sources":[{"name":"A","path":"relative/path"}]}`,
		`{"sources":[{"name":"A","path":"` + js(filepath.Join(f.share, "nope")) + `"}]}`,
		`{"sources":[{"name":"A","path":"` + js(f.share) + `"},{"name":"a","path":"` + js(filme) + `"}]}`,
		`{"sources":[{"name":"A","path":"` + js(f.share) + `"},{"name":"B","path":"` + js(f.share) + `/"}]}`,
		`{"sources":[{"name":"A","path":"` + js(f.share) + `","exclude":["appdata/*/cache"]}]}`,
	} {
		if code := f.call("jdp", "PUT", "/admin/sources", bad, nil); code != 400 {
			t.Errorf("%s: %d, want 400", bad, code)
		}
	}

	body := `{"sources":[{"name":" Shares ","path":"` + js(f.share) + `","indexed":true,"skipHidden":true,"exclude":[" *.tmp ","/appdata/plex/","*.tmp",""]},{"name":"Filme","path":"` + js(filme) + `","readOnly":true}]}`
	if code := f.call("jdp", "PUT", "/admin/sources", body, &out); code != 200 {
		t.Fatalf("put: %d", code)
	}
	if !out.RestartNeeded || len(out.Sources) != 2 {
		t.Fatalf("saved %+v", out)
	}
	s := out.Sources[0]
	if s.Name != "Shares" || !s.SkipHidden || strings.Join(s.Exclude, ",") != "*.tmp,appdata/plex" {
		t.Fatalf("tidied %+v", s)
	}
	if out.Sources[1].Exclude == nil || !out.Sources[1].ReadOnly {
		t.Fatalf("second %+v", out.Sources[1])
	}
	if _, err := os.Stat(f.svc.sourcesFile()); err != nil {
		t.Fatal(err)
	}
	if code := f.call("anna", "PUT", "/admin/sources", body, nil); code != 403 {
		t.Fatalf("user saves sources: %d", code)
	}
}

func js(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }

func TestIntegrationTestUsesTheSavedSecretForTheSavedAddress(t *testing.T) {
	f := newFixture(t)
	if code := f.call("jdp", "PUT", "/admin/app", `{"office":{"kind":"onlyoffice","url":"https://office.lan","secret":"s3"},"arrowloop":{"url":"http://al.lan","token":"tk"}}`, nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	var res IntegrationResult
	for _, tc := range []struct{ body, want string }{
		{`{"kind":"onlyoffice","url":"https://office.lan/"}`, "onlyoffice https://office.lan s3"},
		{`{"kind":"eurooffice","url":"https://other.lan"}`, "eurooffice https://other.lan "},
		{`{"kind":"onlyoffice","url":"https://office.lan","secret":"new"}`, "onlyoffice https://office.lan new"},
		{`{"kind":"arrowloop","url":"http://al.lan"}`, "arrowloop http://al.lan tk"},
		{`{"kind":"collabora","url":"https://cool.lan"}`, "collabora https://cool.lan "},
	} {
		if code := f.call("jdp", "POST", "/admin/integrations/test", tc.body, &res); code != 200 || !res.OK {
			t.Fatalf("%s: %d %+v", tc.body, code, res)
		}
		if got := f.tested[len(f.tested)-1]; got != tc.want {
			t.Errorf("%s: tested %q, want %q", tc.body, got, tc.want)
		}
	}
	for _, bad := range []string{`{"kind":"wordcraft","url":"https://x"}`, `{"kind":"onlyoffice","url":""}`, `{"kind":"onlyoffice","url":"office.lan"}`} {
		if code := f.call("jdp", "POST", "/admin/integrations/test", bad, nil); code != 400 {
			t.Errorf("%s: %d, want 400", bad, code)
		}
	}
}

func TestSystemReportsVersionToolsAndOffice(t *testing.T) {
	f := newFixture(t)
	if code := f.call("jdp", "PUT", "/admin/app", `{"office":{"kind":"collabora","url":"https://cool.lan","secret":"x"}}`, nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	req := httptest.NewRequest("GET", "/system", nil)
	req.Header.Set("X-User", "anna")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"x"`) {
		t.Fatal("the office secret reached a user")
	}
	var in Info
	if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ffmpeg", "ffprobe", "mutool", "7zz", "rclone"} {
		if _, ok := in.Tools[name]; !ok {
			t.Errorf("tool %s missing", name)
		}
	}
	if in.Version == "" || in.Go == "" || in.Feature.Office.Kind != OfficeCollabora || in.Feature.Office.URL != "https://cool.lan" {
		t.Fatalf("info %+v", in)
	}

	// The uptime does not change the ETag.
	again := httptest.NewRequest("GET", "/system", nil)
	again.Header.Set("X-User", "anna")
	again.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec2 := httptest.NewRecorder()
	f.h.ServeHTTP(rec2, again)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("second get: %d", rec2.Code)
	}
}

func TestVersionNumberIsFoundInToolOutput(t *testing.T) {
	for out, want := range map[string]string{
		"ffmpeg version 7.1.1-static https://johnvansickle.com Copyright": "7.1.1",
		"ffmpeg version n7.1 Copyright (c) 2000-2024":                     "7.1",
		"mutool version 1.24.0":                                           "1.24.0",
		"\n7-Zip (z) 24.08 (x64) : Copyright (c) 1999-2024 Igor Pavlov":   "24.08",
		"rclone v1.75.2\n- os/version: debian 13.7":                       "1.75.2",
	} {
		if got := versionNumber.FindString(out); got != want {
			t.Errorf("%q: %q, want %q", out, got, want)
		}
	}
}
