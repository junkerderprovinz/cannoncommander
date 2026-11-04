package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	_ "modernc.org/sqlite"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

type fixture struct {
	t      *testing.T
	h      http.Handler
	share  string
	office system.Office
	people map[string]*users.User
}

const base = "http://cc.test/api/v1"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	share := filepath.Join(dir, "user")
	if err := os.MkdirAll(filepath.Join(share, "dokumente"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(share, "dokumente", "Brief.docx"), []byte("alt"), 0o640); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "main.sqlite"))
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
	f := &fixture{t: t, share: share, people: map[string]*users.User{
		"jdp":  {ID: 1, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: all}}},
		"anna": {ID: 2, BackendScopes: []users.BackendScope{{Path: share, Scope: "/", Permissions: users.SourceFilePermissions{View: true, Download: true}}}},
	}}
	for name, u := range f.people {
		u.Username = name
	}
	byID := func(id uint64) (*users.User, error) {
		for _, u := range f.people {
			if u.ID == id {
				return u, nil
			}
		}
		return nil, os.ErrNotExist
	}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := f.people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	router := v1.NewRouter(v1.Deps{FS: fs, DB: db}, auth)
	router.Contract("office", contract)
	bind(router, &service{
		fs:        fs,
		keys:      &keys{db: db},
		settings:  func(context.Context) (system.Office, error) { return f.office, nil },
		user:      byID,
		base:      func(*http.Request) string { return base },
		discovery: &discovery{},
		locks:     newLocks(),
		client:    http.DefaultClient,
	})
	f.h, err = router.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) config(who string) (int, Config) {
	f.t.Helper()
	req := httptest.NewRequest("GET", "/office/config?path=/mnt/user/dokumente/Brief.docx", nil)
	req.Header.Set("X-User", who)
	rec := f.do(req)
	var c Config
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
			f.t.Fatal(err)
		}
	}
	return rec.Code, c
}

// local turns an address the document server would call into a request
// for the handler.
func local(t *testing.T, method, raw string, body io.Reader) *http.Request {
	t.Helper()
	if !strings.HasPrefix(raw, base) {
		t.Fatalf("%s is not below %s", raw, base)
	}
	return httptest.NewRequest(method, strings.TrimPrefix(raw, base), body)
}

func (f *fixture) content() string {
	b, err := os.ReadFile(filepath.Join(f.share, "dokumente", "Brief.docx"))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}

func sign(t *testing.T, secret string, claims jwt.MapClaims) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOnlyOfficeOpensFetchesAndSavesWithTheSecret(t *testing.T) {
	f := newFixture(t)
	if code, _ := f.config("jdp"); code != http.StatusConflict {
		t.Fatalf("without an office server: %d", code)
	}
	docServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("neu")) }))
	defer docServer.Close()
	f.office = system.Office{Kind: system.OfficeOnlyOffice, URL: docServer.URL, Secret: "geheim"}

	code, c := f.config("jdp")
	if code != 200 || c.Kind != "onlyoffice" || c.URL != docServer.URL {
		t.Fatalf("config: %d %+v", code, c)
	}
	signed, _ := c.Config["token"].(string)
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(signed, claims, func(*jwt.Token) (any, error) { return []byte("geheim"), nil }); err != nil {
		t.Fatalf("config not signed with the secret: %v", err)
	}
	doc := c.Config["document"].(map[string]any)
	editor := c.Config["editorConfig"].(map[string]any)
	if c.Config["documentType"] != "word" || editor["mode"] != "edit" || doc["title"] != "Brief.docx" || doc["key"] == "" {
		t.Fatalf("editor config %+v", c.Config)
	}

	rec := f.do(local(t, "GET", doc["url"].(string), nil))
	if rec.Code != 200 || rec.Body.String() != "alt" {
		t.Fatalf("download: %d %q", rec.Code, rec.Body)
	}
	callback := editor["callbackUrl"].(string)
	post := func(body map[string]any, header string) int {
		b, _ := json.Marshal(body)
		req := local(t, "POST", callback, strings.NewReader(string(b)))
		if header != "" {
			req.Header.Set("Authorization", "Bearer "+header)
		}
		rec := f.do(req)
		var out map[string]int
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out["error"]
	}

	if post(map[string]any{"key": doc["key"], "status": 2, "url": docServer.URL + "/cache/out.docx"}, "") != 1 || f.content() != "alt" {
		t.Fatal("an unsigned callback was taken")
	}
	elsewhere := jwt.MapClaims{"key": doc["key"], "status": 2, "url": "http://169.254.169.254/latest"}
	if post(map[string]any{"token": sign(t, "geheim", elsewhere)}, "") != 1 || f.content() != "alt" {
		t.Fatal("a document from another host was fetched")
	}
	if post(map[string]any{"key": doc["key"], "status": 1}, "") != 1 {
		t.Fatal("an unsigned status was taken")
	}
	inHeader := sign(t, "geheim", jwt.MapClaims{"payload": map[string]any{"key": doc["key"], "status": 1}})
	if post(map[string]any{"key": doc["key"], "status": 1}, inHeader) != 0 {
		t.Fatal("a status signed in the header was refused")
	}
	saved := jwt.MapClaims{"key": doc["key"], "status": 2, "url": docServer.URL + "/cache/out.docx"}
	if post(map[string]any{"token": sign(t, "geheim", saved)}, "") != 0 || f.content() != "neu" {
		t.Fatalf("save: %q", f.content())
	}
	if info, _ := os.Stat(filepath.Join(f.share, "dokumente", "Brief.docx")); runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", info.Mode())
	}
	if _, c2 := f.config("jdp"); c2.Config["document"].(map[string]any)["key"] == doc["key"] {
		t.Fatal("the document key stayed after a change")
	}
}

func TestReadOnlyUsersGetAViewerThatCannotSave(t *testing.T) {
	f := newFixture(t)
	docServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("neu")) }))
	defer docServer.Close()
	f.office = system.Office{Kind: system.OfficeEuroOffice, URL: docServer.URL}
	_, c := f.config("anna")
	editor := c.Config["editorConfig"].(map[string]any)
	if editor["mode"] != "view" || c.Config["token"] != nil {
		t.Fatalf("config %+v", c.Config)
	}
	body := `{"key":"k","status":2,"url":"` + docServer.URL + `/out.docx"}`
	rec := f.do(local(t, "POST", editor["callbackUrl"].(string), strings.NewReader(body)))
	if !strings.Contains(rec.Body.String(), `"error":1`) || f.content() != "alt" {
		t.Fatalf("a read only user saved: %s", rec.Body)
	}
}

func TestATakenRightClosesTheDocument(t *testing.T) {
	f := newFixture(t)
	f.office = system.Office{Kind: system.OfficeOnlyOffice, URL: "https://office.lan"}
	_, c := f.config("jdp")
	file := c.Config["document"].(map[string]any)["url"].(string)
	f.people["jdp"].BackendScopes = nil
	if rec := f.do(local(t, "GET", file, nil)); rec.Code != http.StatusForbidden {
		t.Fatalf("download after the right was taken: %d", rec.Code)
	}
	if rec := f.do(local(t, "GET", strings.Replace(file, "token=", "token=x", 1), nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token: %d", rec.Code)
	}
}

const discoveryXML = `<?xml version="1.0" encoding="utf-8"?>
<wopi-discovery><net-zone name="external-http">
 <app name="writer"><action name="edit" ext="docx" urlsrc="https://cool.lan/browser/abc/cool.html?&lt;ui=UI_LLCC&amp;&gt;"/>
  <action name="view" ext="pdf" urlsrc="https://cool.lan/browser/abc/cool.html?"/></app>
</net-zone></wopi-discovery>`

func TestCollaboraEditsThroughWOPIWithLocks(t *testing.T) {
	f := newFixture(t)
	cool := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hosting/discovery" {
			_, _ = w.Write([]byte(discoveryXML))
		}
	}))
	defer cool.Close()
	f.office = system.Office{Kind: system.OfficeCollabora, URL: cool.URL}
	code, c := f.config("jdp")
	if code != 200 || c.Token == "" || c.TTL < time.Now().Add(time.Hour).UnixMilli() {
		t.Fatalf("config: %d %+v", code, c)
	}
	prefix := "https://cool.lan/browser/abc/cool.html?WOPISrc="
	if !strings.HasPrefix(c.URL, prefix) {
		t.Fatalf("editor address %s", c.URL)
	}
	src, err := url.QueryUnescape(strings.TrimPrefix(c.URL, prefix))
	if err != nil {
		t.Fatal(err)
	}
	q := "?access_token=" + url.QueryEscape(c.Token)
	wopi := func(method, suffix, body string, header ...string) *httptest.ResponseRecorder {
		req := local(t, method, src+suffix+q, strings.NewReader(body))
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		return f.do(req)
	}

	rec := wopi("GET", "", "")
	var info map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if rec.Code != 200 || info["BaseFileName"] != "Brief.docx" || info["UserCanWrite"] != true || info["Size"] != float64(3) {
		t.Fatalf("CheckFileInfo: %d %v", rec.Code, info)
	}
	if rec := wopi("GET", "/contents", ""); rec.Body.String() != "alt" {
		t.Fatalf("GetFile: %q", rec.Body)
	}
	if rec := wopi("POST", "", "", "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "A"); rec.Code != 200 {
		t.Fatalf("lock: %d", rec.Code)
	}
	if rec := wopi("POST", "", "", "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "B"); rec.Code != 409 || rec.Header().Get("X-WOPI-Lock") != "A" {
		t.Fatalf("second lock: %d %q", rec.Code, rec.Header().Get("X-WOPI-Lock"))
	}
	if rec := wopi("POST", "/contents", "falsch", "X-WOPI-Override", "PUT", "X-WOPI-Lock", "B"); rec.Code != 409 || f.content() != "alt" {
		t.Fatalf("save with the wrong lock: %d", rec.Code)
	}
	if rec := wopi("POST", "/contents", "neu", "X-WOPI-Override", "PUT", "X-WOPI-Lock", "A"); rec.Code != 200 || f.content() != "neu" || rec.Header().Get("X-WOPI-ItemVersion") == "" {
		t.Fatalf("save: %d %q", rec.Code, f.content())
	}
	if rec := wopi("POST", "", "", "X-WOPI-Override", "GET_LOCK"); rec.Header().Get("X-WOPI-Lock") != "A" {
		t.Fatalf("get lock: %q", rec.Header().Get("X-WOPI-Lock"))
	}
	if rec := wopi("POST", "", "", "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "C", "X-WOPI-OldLock", "A"); rec.Code != 200 {
		t.Fatalf("relock: %d", rec.Code)
	}
	if rec := wopi("POST", "", "", "X-WOPI-Override", "UNLOCK", "X-WOPI-Lock", "C"); rec.Code != 200 {
		t.Fatalf("unlock: %d", rec.Code)
	}
	stale := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if rec := wopi("POST", "/contents", "alt", "X-WOPI-Override", "PUT", "X-COOL-WOPI-Timestamp", stale); rec.Code != 409 || !strings.Contains(rec.Body.String(), "1010") {
		t.Fatalf("save over a newer file: %d %s", rec.Code, rec.Body)
	}

	other := local(t, "GET", base+"/wopi/files/"+fileID("/mnt/user/other.docx")+q, nil)
	if rec := f.do(other); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token used for another file: %d", rec.Code)
	}
}

func TestCollaboraViewersCannotLockOrSave(t *testing.T) {
	f := newFixture(t)
	cool := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(discoveryXML)) }))
	defer cool.Close()
	f.office = system.Office{Kind: system.OfficeCollabora, URL: cool.URL}
	_, c := f.config("anna")
	src := base + "/wopi/files/" + fileID("/mnt/user/dokumente/Brief.docx")
	q := "?access_token=" + url.QueryEscape(c.Token)
	req := local(t, "POST", src+q, nil)
	req.Header.Set("X-WOPI-Override", "LOCK")
	req.Header.Set("X-WOPI-Lock", "A")
	if rec := f.do(req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("viewer locks: %d", rec.Code)
	}
	if rec := f.do(local(t, "POST", src+"/contents"+q, strings.NewReader("x"))); rec.Code != http.StatusUnauthorized || f.content() != "alt" {
		t.Fatalf("viewer saves: %d", rec.Code)
	}
}
