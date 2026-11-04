package prefs_test

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
	"time"

	_ "modernc.org/sqlite"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
)

type fixture struct {
	t     *testing.T
	h     http.Handler
	db    *sql.DB
	hub   *hub.Hub
	share string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	share := filepath.Join(base, "user")
	for _, d := range []string{"downloads", "filme", "fotos/2026"} {
		if err := os.MkdirAll(filepath.Join(share, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(share, "downloads", "notes.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: share})
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(base, "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}

	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	people := map[string]*users.User{
		"jdp":  {ID: 1, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {ID: 2, BackendScopes: []users.BackendScope{{Path: share, Scope: "/fotos", Permissions: all}}},
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
	events := hub.New(hub.Options{})
	r := v1.NewRouter(v1.Deps{FS: fs, Hub: events, DB: db}, auth)
	search.Routes(r)
	prefs.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, h: h, db: db, hub: events, share: share}
}

func (f *fixture) do(user, method, target string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var payload *strings.Reader
	switch b := body.(type) {
	case nil:
		payload = strings.NewReader("")
	case string:
		payload = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			f.t.Fatal(err)
		}
		payload = strings.NewReader(string(raw))
	}
	req := httptest.NewRequest(method, target, payload)
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) must(want int, user, method, target string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	rec := f.do(user, method, target, body)
	if rec.Code != want {
		f.t.Fatalf("%s %s as %s: %d, want %d: %s", method, target, user, rec.Code, want, rec.Body)
	}
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

func TestEachDeviceKeepsItsOwnLayoutWhileTheRestIsShared(t *testing.T) {
	f := newFixture(t)
	f.must(204, "jdp", "PUT", "/me/prefs?device=desk", map[string]any{"theme": "light", "dual": true, "colWidths": map[string]int{"name": 300}})
	f.must(204, "jdp", "PUT", "/me/prefs?device=phone", map[string]any{"theme": "dark", "dual": false, "fkeys": false})

	desk := decode[map[string]any](t, f.must(200, "jdp", "GET", "/me/prefs?device=desk", nil))
	if desk["theme"] != "dark" || desk["dual"] != true || desk["colWidths"] == nil {
		t.Errorf("desk = %v", desk)
	}
	phone := decode[map[string]any](t, f.must(200, "jdp", "GET", "/me/prefs?device=phone", nil))
	if phone["theme"] != "dark" || phone["dual"] != false || phone["colWidths"] != nil {
		t.Errorf("phone = %v", phone)
	}
	plain := decode[map[string]any](t, f.must(200, "jdp", "GET", "/me/prefs", nil))
	if _, ok := plain["dual"]; ok || plain["theme"] != "dark" {
		t.Errorf("without device = %v", plain)
	}
}

func TestSettingsSavedWithoutDeviceServeEveryDevice(t *testing.T) {
	f := newFixture(t)
	f.must(204, "jdp", "PUT", "/me/prefs", map[string]any{"dual": false, "side": false})
	got := decode[map[string]any](t, f.must(200, "jdp", "GET", "/me/prefs?device=new", nil))
	if got["dual"] != false || got["side"] != false {
		t.Errorf("got %v", got)
	}
}

func TestSettingsBelongToOneUser(t *testing.T) {
	f := newFixture(t)
	f.must(204, "jdp", "PUT", "/me/prefs", map[string]any{"theme": "light"})
	got := decode[map[string]any](t, f.must(200, "anna", "GET", "/me/prefs", nil))
	if len(got) != 0 {
		t.Errorf("anna sees %v", got)
	}
}

func TestSettingsKeepTheirTypes(t *testing.T) {
	f := newFixture(t)
	f.must(400, "jdp", "PUT", "/me/prefs", map[string]any{"dual": "yes"})
	f.must(400, "jdp", "PUT", "/me/prefs", map[string]any{"trashDays": nil})
	f.must(400, "jdp", "PUT", "/me/prefs", "[]")
	f.must(400, "jdp", "PUT", "/me/prefs", "null")
	f.must(400, "jdp", "GET", "/me/prefs?device=not%20ok", nil)
	f.must(413, "jdp", "PUT", "/me/prefs", map[string]any{"notes": strings.Repeat("x", 300<<10)})
	f.must(204, "jdp", "PUT", "/me/prefs", map[string]any{"newsSeen": 3, "accentPresets": []string{"#fcc419"}})
	f.must(401, "", "GET", "/me/prefs", nil)
}

func TestSavingSettingsTellsOnlyThatUser(t *testing.T) {
	f := newFixture(t)
	mine, _, _ := f.hub.Connect("tab-1", "jdp", 0)
	other, _, _ := f.hub.Connect("tab-2", "anna", 0)
	defer f.hub.Disconnect(mine)
	defer f.hub.Disconnect(other)

	f.must(204, "jdp", "PUT", "/me/prefs?device=desk", map[string]any{"theme": "light"})
	select {
	case e := <-mine.Events:
		if e.Type != prefs.EventChanged || !strings.Contains(string(e.Data), `"kind":"settings"`) || !strings.Contains(string(e.Data), `"device":"desk"`) {
			t.Errorf("event %s %s", e.Type, e.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no prefs.changed for jdp")
	}
	select {
	case e := <-other.Events:
		t.Errorf("anna got %s %s", e.Type, e.Data)
	default:
	}
}

type bookmarkList struct {
	Items []prefs.Bookmark `json:"items"`
}

func TestNewUsersStartWithTheSharesTheyCanOpen(t *testing.T) {
	f := newFixture(t)
	names := func(user string) []string {
		var out []string
		for _, b := range decode[bookmarkList](t, f.must(200, user, "GET", "/me/bookmarks", nil)).Items {
			out = append(out, b.Name)
		}
		return out
	}
	if got := strings.Join(names("jdp"), ","); got != "Downloads,Filme,Fotos" {
		t.Errorf("jdp starts with %s", got)
	}
	if got := strings.Join(names("anna"), ","); got != "Fotos" {
		t.Errorf("anna starts with %s", got)
	}

	f.must(204, "jdp", "PUT", "/me/bookmarks", bookmarkList{Items: []prefs.Bookmark{}})
	if got := names("jdp"); len(got) != 0 {
		t.Errorf("an emptied list came back as %v", got)
	}
}

func TestBookmarksKeepTheirOrder(t *testing.T) {
	f := newFixture(t)
	list := bookmarkList{Items: []prefs.Bookmark{
		{Name: " Fotos 2026 ", Path: "/mnt/user/fotos/2026/", Glyph: "IconFileImage"},
		{Name: "Downloads", Path: "/mnt/user/downloads"},
	}}
	f.must(204, "jdp", "PUT", "/me/bookmarks", list)
	got := decode[bookmarkList](t, f.must(200, "jdp", "GET", "/me/bookmarks", nil)).Items
	if len(got) != 2 || got[0] != (prefs.Bookmark{Name: "Fotos 2026", Path: "/mnt/user/fotos/2026", Glyph: "IconFileImage"}) || got[1].Path != "/mnt/user/downloads" {
		t.Errorf("got %+v", got)
	}

	twice := bookmarkList{Items: []prefs.Bookmark{{Name: "a", Path: "/mnt/user/filme"}, {Name: "b", Path: "/mnt/user/filme/"}}}
	f.must(409, "jdp", "PUT", "/me/bookmarks", twice)
	f.must(400, "jdp", "PUT", "/me/bookmarks", bookmarkList{Items: []prefs.Bookmark{{Name: "zip", Path: "/mnt/user/a.zip::/x"}}})
	f.must(400, "jdp", "PUT", "/me/bookmarks", bookmarkList{Items: []prefs.Bookmark{{Name: " ", Path: "/mnt/user/filme"}}})
	f.must(400, "jdp", "PUT", "/me/bookmarks", bookmarkList{Items: []prefs.Bookmark{{Name: "up", Path: "/mnt/user/../etc"}}})
}

type presetList struct {
	Items []prefs.RenamePreset `json:"items"`
}

func TestRenamePresetsAreSavedAsAList(t *testing.T) {
	f := newFixture(t)
	if got := decode[presetList](t, f.must(200, "jdp", "GET", "/me/rename-presets", nil)).Items; len(got) != 0 {
		t.Fatalf("fresh user has presets %v", got)
	}
	list := presetList{Items: []prefs.RenamePreset{
		{Name: "Urlaub", Options: prefs.RenameOptions{Name: "Urlaub_[C3]", Ext: "[E]"}},
		{Name: "Klein", Options: prefs.RenameOptions{Name: "[N]", Ext: "[E]", Case: "lower"}},
	}}
	f.must(204, "jdp", "PUT", "/me/rename-presets", list)
	got := decode[presetList](t, f.must(200, "jdp", "GET", "/me/rename-presets", nil)).Items
	if len(got) != 2 || got[0].Options.Case != "keep" || got[1].Options.Case != "lower" || got[0].Options.Name != "Urlaub_[C3]" {
		t.Errorf("got %+v", got)
	}

	list.Items[1].Name = "Urlaub"
	f.must(409, "jdp", "PUT", "/me/rename-presets", list)
	list.Items[1] = prefs.RenamePreset{Name: "Laut", Options: prefs.RenameOptions{Case: "shout"}}
	f.must(400, "jdp", "PUT", "/me/rename-presets", list)
}

func TestProfilesBelongToTheirOwner(t *testing.T) {
	f := newFixture(t)
	in := prefs.ProfileInput{
		Name: "Fotos sortieren", Dual: true, Columns: []string{"name", "size"},
		Panels: []prefs.ProfilePanel{{Cur: 1, Tabs: []string{"/mnt/user/downloads", "/mnt/user/fotos/"}}, {Cur: 0, Tabs: []string{"/mnt/user/fotos/2026"}}},
	}
	created := decode[prefs.Profile](t, f.must(201, "jdp", "POST", "/me/profiles", in))
	if created.ID == 0 || created.Panels[0].Tabs[1] != "/mnt/user/fotos" {
		t.Fatalf("created %+v", created)
	}
	in.Name = "Nur Downloads"
	in.Dual = false
	in.Panels = in.Panels[:1]
	target := "/me/profiles/" + jsonNumber(created.ID)
	f.must(404, "anna", "PUT", target, in)
	f.must(404, "anna", "DELETE", target, nil)
	f.must(200, "jdp", "PUT", target, in)

	list := decode[struct{ Items []prefs.Profile }](t, f.must(200, "jdp", "GET", "/me/profiles", nil)).Items
	if len(list) != 1 || list[0].Name != "Nur Downloads" || list[0].Dual || len(list[0].Panels) != 1 {
		t.Errorf("list %+v", list)
	}
	if others := decode[struct{ Items []prefs.Profile }](t, f.must(200, "anna", "GET", "/me/profiles", nil)).Items; len(others) != 0 {
		t.Errorf("anna sees %+v", others)
	}

	f.must(204, "jdp", "DELETE", target, nil)
	f.must(404, "jdp", "DELETE", target, nil)
	f.must(404, "jdp", "DELETE", "/me/profiles/abc", nil)
}

func TestProfilesNeedAValidLayout(t *testing.T) {
	f := newFixture(t)
	tab := []string{"/mnt/user"}
	for name, in := range map[string]prefs.ProfileInput{
		"no name":        {Panels: []prefs.ProfilePanel{{Tabs: tab}}},
		"no panel":       {Name: "x"},
		"three panels":   {Name: "x", Panels: []prefs.ProfilePanel{{Tabs: tab}, {Tabs: tab}, {Tabs: tab}}},
		"no tab":         {Name: "x", Panels: []prefs.ProfilePanel{{}}},
		"cur out of tab": {Name: "x", Panels: []prefs.ProfilePanel{{Cur: 1, Tabs: tab}}},
		"relative tab":   {Name: "x", Panels: []prefs.ProfilePanel{{Tabs: []string{"mnt/user"}}}},
	} {
		if rec := f.do("jdp", "POST", "/me/profiles", in); rec.Code != 400 {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
}

func jsonNumber(n int64) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

func TestSavedSearchesKeepTheQueryAsSent(t *testing.T) {
	f := newFixture(t)
	query := `{"text":"","in":"/mnt/user","name":"*.iso *.img","min":"1 GB","when":"any","type":"all","hidden":false,"future":{"x":1}}`
	created := decode[prefs.SavedSearch](t, f.must(201, "jdp", "POST", "/search/saved", `{"name":"Große Abbilder","query":`+query+`}`))
	got := decode[struct{ Items []prefs.SavedSearch }](t, f.must(200, "jdp", "GET", "/search/saved", nil)).Items
	if len(got) != 1 || got[0].ID != created.ID || got[0].Name != "Große Abbilder" || !strings.Contains(string(got[0].Query), `"future":{"x":1}`) {
		t.Fatalf("got %+v", got)
	}

	target := "/search/saved/" + jsonNumber(created.ID)
	f.must(200, "jdp", "PUT", target, `{"name":"Abbilder","query":{"in":"/mnt/user/downloads"}}`)
	f.must(404, "anna", "DELETE", target, nil)
	f.must(400, "jdp", "PUT", target, `{"name":"x","query":"*.iso"}`)
	f.must(400, "jdp", "PUT", target, `{"name":"x","query":{"in":"../etc"}}`)
	f.must(400, "jdp", "POST", "/search/saved", `{"name":"","query":{}}`)
	f.must(204, "jdp", "DELETE", target, nil)
	if got := decode[struct{ Items []prefs.SavedSearch }](t, f.must(200, "jdp", "GET", "/search/saved", nil)).Items; len(got) != 0 {
		t.Errorf("left over %+v", got)
	}
}

type frequentList struct {
	Items []prefs.Folder `json:"items"`
}

func (f *fixture) top(user, query string) []string {
	f.t.Helper()
	var out []string
	for _, d := range decode[frequentList](f.t, f.must(200, user, "GET", "/me/frequent"+query, nil)).Items {
		out = append(out, d.Path)
	}
	return out
}

func TestFrequentFoldersRankByVisitsAndRecency(t *testing.T) {
	f := newFixture(t)
	visit := func(p string, n int) {
		for range n {
			f.must(204, "jdp", "POST", "/me/frequent", map[string]string{"path": p})
		}
	}
	visit("/mnt/user/downloads", 3)
	visit("/mnt/user/filme", 2)
	visit("/mnt/user/fotos/2026", 1)
	if got := strings.Join(f.top("jdp", ""), " "); got != "/mnt/user/downloads /mnt/user/filme /mnt/user/fotos/2026" {
		t.Errorf("ranking %s", got)
	}
	if got := strings.Join(f.top("jdp", "?limit=1&skip=/mnt/user/downloads"), " "); got != "/mnt/user/filme" {
		t.Errorf("limit and skip %s", got)
	}

	// Three visits a month ago weigh less than one visit just now.
	month := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := f.db.Exec(`UPDATE frequent_dirs SET last_at = ? WHERE path = '/mnt/user/downloads'`, month); err != nil {
		t.Fatal(err)
	}
	if got := f.top("jdp", ""); got[len(got)-1] != "/mnt/user/downloads" {
		t.Errorf("old visits still lead: %v", got)
	}

	if err := os.RemoveAll(filepath.Join(f.share, "filme")); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.top("jdp", "") {
		if p == "/mnt/user/filme" {
			t.Error("a deleted folder is still offered")
		}
	}
	if got := f.top("anna", ""); len(got) != 0 {
		t.Errorf("anna sees %v", got)
	}
}

func TestOnlyFoldersOneMayOpenAreCounted(t *testing.T) {
	f := newFixture(t)
	f.must(400, "jdp", "POST", "/me/frequent", map[string]string{"path": "/mnt/user/downloads/notes.md"})
	f.must(404, "jdp", "POST", "/me/frequent", map[string]string{"path": "/mnt/user/nowhere"})
	f.must(403, "anna", "POST", "/me/frequent", map[string]string{"path": "/mnt/user/downloads"})
	f.must(204, "jdp", "POST", "/me/frequent", map[string]string{"path": "/mnt/user/downloads/a.zip::/inner"})
	f.must(400, "jdp", "GET", "/me/frequent?limit=0", nil)
	if got := f.top("jdp", ""); len(got) != 0 {
		t.Errorf("counted %v", got)
	}
}

func TestForgetDeletesEverythingOfOneUser(t *testing.T) {
	f := newFixture(t)
	for _, user := range []string{"jdp", "anna"} {
		f.must(204, user, "PUT", "/me/prefs?device=desk", map[string]any{"theme": "light", "dual": true})
		f.must(204, user, "PUT", "/me/rename-presets", presetList{Items: []prefs.RenamePreset{{Name: "Klein", Options: prefs.RenameOptions{Name: "[N]", Ext: "[E]", Case: "lower"}}}})
		f.must(201, user, "POST", "/search/saved", `{"name":"Bilder","query":{"in":"/mnt/user"}}`)
	}
	if err := prefs.Forget(context.Background(), f.db, 2); err != nil {
		t.Fatal(err)
	}
	if got := decode[map[string]any](t, f.must(200, "anna", "GET", "/me/prefs?device=desk", nil)); len(got) != 0 {
		t.Errorf("anna still has settings: %v", got)
	}
	if got := decode[map[string]any](t, f.must(200, "jdp", "GET", "/me/prefs?device=desk", nil)); got["theme"] != "light" {
		t.Errorf("jdp lost his settings: %v", got)
	}
	var left int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM saved_searches WHERE user_id = '2'`).Scan(&left); err != nil || left != 0 {
		t.Errorf("anna has %d saved searches left, %v", left, err)
	}
}

func TestLanguageIsWhatTheUserPicked(t *testing.T) {
	f := newFixture(t)
	got, err := prefs.Language(context.Background(), f.db, 7)
	if err != nil || got != "" {
		t.Fatalf("before a pick: %q %v", got, err)
	}
	if _, err := f.db.Exec(`INSERT INTO user_prefs (user_id, data, updated_at) VALUES ('7', '{"language":"en"}', 0)`); err != nil {
		t.Fatal(err)
	}
	got, err = prefs.Language(context.Background(), f.db, 7)
	if err != nil || got != "en" {
		t.Fatalf("after a pick: %q %v", got, err)
	}
}
