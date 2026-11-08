package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
)

func TestExportLeavesSecretsOutAndImportBringsSettingsBack(t *testing.T) {
	f := newFixture(t)
	var people []string
	RegisterSection(Section{
		Name: "users", Order: 30, People: true,
		Export: func(context.Context, v1.Deps) (any, error) { return []string{"anna"}, nil },
		Import: func(_ context.Context, _ v1.Deps, raw json.RawMessage) ([]string, error) {
			return nil, json.Unmarshal(raw, &people)
		},
	})
	if code := f.call("jdp", "PUT", "/admin/app", `{"office":{"kind":"onlyoffice","url":"https://office.lan","secret":"geheim"},"logLevel":"debug"}`, nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	target := "/mnt/user/appdata/cannoncommander/exports/settings.json"
	var written struct {
		Path string `json:"path"`
	}
	if code := f.call("jdp", "POST", "/admin/app/export", `{"path":"`+target+`","users":false}`, &written); code != 201 || written.Path != target {
		t.Fatalf("export: %d %+v", code, written)
	}
	data, err := os.ReadFile(filepath.Join(f.share, "appdata", "cannoncommander", "exports", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "geheim") {
		t.Fatal("the secret went into the file")
	}
	var file exportFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Sections["users"]; ok {
		t.Fatal("users went along without being asked")
	}
	for _, name := range []string{"app", "sources"} {
		if _, ok := file.Sections[name]; !ok {
			t.Errorf("section %s missing", name)
		}
	}
	if code := f.call("jdp", "POST", "/admin/app/export", `{"path":"`+target+`"}`, nil); code != http.StatusConflict {
		t.Fatalf("second export: %d", code)
	}
	for _, bad := range []string{
		`{"path":"/mnt/user/appdata/settings.txt"}`,
		`{"path":"/mnt/settings.json"}`,
		`{"path":"/mnt/user/nope/settings.json"}`,
	} {
		if code := f.call("jdp", "POST", "/admin/app/export", bad, nil); code < 400 {
			t.Errorf("%s: %d", bad, code)
		}
	}

	if code := f.call("jdp", "PUT", "/admin/app", `{"logLevel":"error","office":{"url":"https://other.lan"}}`, nil); code != 200 {
		t.Fatalf("change: %d", code)
	}
	var res ImportResult
	if code := f.call("jdp", "POST", "/admin/app/import", `{"path":"`+target+`","users":true}`, &res); code != 200 {
		t.Fatalf("import: %d", code)
	}
	if strings.Join(res.Sections, ",") != "sources,app" || res.RestartNeeded {
		t.Fatalf("import result %+v", res)
	}
	got, err := f.svc.store.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.LogLevel != "debug" || got.Office.URL != "https://office.lan" || got.Office.Secret != "geheim" {
		t.Fatalf("after import %+v", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/app/export?users=true", nil)
	req.Header.Set("X-User", "jdp")
	f.h.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "cannoncommander-settings-") {
		t.Fatalf("download: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	if err := os.WriteFile(filepath.Join(f.share, "with-users.json"), rec.Body.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.call("jdp", "POST", "/admin/app/import", `{"path":"/mnt/user/with-users.json","users":true}`, &res); code != 200 || len(people) != 1 {
		t.Fatalf("import with users: %d %v", code, people)
	}

	if err := os.WriteFile(filepath.Join(f.share, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.call("jdp", "POST", "/admin/app/import", `{"path":"/mnt/user/package.json"}`, nil); code != 400 {
		t.Fatalf("foreign file: %d", code)
	}
	if code := f.call("anna", "POST", "/admin/app/import", `{"path":"`+target+`"}`, nil); code != 403 {
		t.Fatalf("user imports: %d", code)
	}
}

func TestUpdateCheckComparesWithTheNewestRelease(t *testing.T) {
	f := newFixture(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"tag_name":"v99.1.0","html_url":"https://github.com/junkerderprovinz/cannoncommander/releases/tag/v99.1.0"}`))
	}))
	defer srv.Close()
	f.svc.releases = srv.URL
	var out UpdateCheck
	for range 2 {
		if code := f.call("anna", "POST", "/system/update", "", &out); code != 200 {
			t.Fatalf("update: %d", code)
		}
	}
	if out.Latest != "99.1.0" || out.ReleaseURL == "" || calls != 1 {
		t.Fatalf("%+v after %d calls", out, calls)
	}

	none := httptest.NewServer(http.NotFoundHandler())
	defer none.Close()
	f.svc.releases, f.svc.updates.at = none.URL, f.svc.updates.at.AddDate(0, 0, -1)
	if code := f.call("anna", "POST", "/system/update", "", &out); code != 200 || out.UpdateAvailable || out.Latest != out.Version {
		t.Fatalf("without releases: %d %+v", code, out)
	}
}

func TestOnlyAReleaseIsOfferedAnUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.4.0","html_url":"https://github.com/junkerderprovinz/cannoncommander/releases/tag/v1.4.0"}`))
	}))
	defer srv.Close()
	saved := version.Version
	t.Cleanup(func() { version.Version = saved })
	for current, want := range map[string]bool{
		"v1.3.2":            true,
		"1.3.2":             true,
		"v1.4.0":            false,
		"v1.4.0-3-gabc1234": false,
		"dev-abc1234":       false,
		"dev":               false,
	} {
		version.Version = current
		f := newFixture(t)
		f.svc.releases = srv.URL
		var out UpdateCheck
		if code := f.call("anna", "POST", "/system/update", "", &out); code != 200 || out.UpdateAvailable != want || out.Latest != "1.4.0" {
			t.Errorf("%s: %d %+v", current, code, out)
		}
	}
}
