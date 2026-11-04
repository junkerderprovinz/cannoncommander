package shares_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type env struct {
	t      testing.TB
	root   string
	deps   v1.Deps
	srv    *httptest.Server
	people map[string]*users.User

	mu      sync.Mutex
	notices []unraid.Notice
}

var full = users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}

func newEnv(t testing.TB) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{t: t, root: root}
	e.write("/mnt/user/fotos/Urlaub/strand.jpg", "sonne")
	e.write("/mnt/user/fotos/Urlaub/2025/meer.jpg", "wellen")
	e.write("/mnt/user/fotos/Urlaub/.geheim", "psst")
	e.write("/mnt/user/fotos/a/IMG.jpg", "eins")
	e.write("/mnt/user/fotos/b/IMG.jpg", "zwei")
	e.write("/mnt/user/dokumente/rechnung.pdf", "%PDF rechnung")
	e.write("/mnt/user/privat/tagebuch.txt", "privat")
	if err := os.MkdirAll(e.real("/mnt/user/eingang"), 0o777); err != nil {
		t.Fatal(err)
	}

	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	user := func(id uint64, name string, admin, share bool, scope string, perms users.SourceFilePermissions) *users.User {
		u := &users.User{ID: id, BackendScopes: []users.BackendScope{{Path: root, Scope: scope, Permissions: perms}}}
		u.Username = name
		u.Permissions.Admin, u.Permissions.Share = admin, share
		return u
	}
	// Real ids are random, so the one of anna lies above the int64 range.
	e.people = map[string]*users.User{
		"jdp":  user(1, "jdp", true, true, "/", full),
		"anna": user(1<<63|2, "anna", false, true, "/", full),
		"gast": user(3, "gast", false, false, "/", users.SourceFilePermissions{View: true, Download: true}),
		"bob":  user(4, "bob", false, true, "/", users.SourceFilePermissions{View: true, Download: true}),
		"eva":  user(5, "eva", false, true, "/dokumente", full),
	}
	byID, notify := shares.UserByID, shares.Notify
	shares.UserByID = func(id uint64) (*users.User, error) {
		for _, u := range e.people {
			if u.ID == id {
				return u, nil
			}
		}
		return nil, errors.New("no such user")
	}
	shares.Notify = func(n unraid.Notice) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.notices = append(e.notices, n)
		return nil
	}
	e.deps = v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t)}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := e.people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	r := v1.NewRouter(e.deps, auth)
	fileops.Routes(r)
	jobs.Routes(r)
	trash.Routes(r)
	shares.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(h)
	t.Cleanup(func() {
		e.srv.Close()
		if s := shares.For(e.deps); s != nil {
			s.Close()
		}
		if m := jobs.For(e.deps); m != nil {
			m.Close()
		}
		if s := trash.For(e.deps); s != nil {
			s.Close()
		}
		for _, name := range []string{"shares", "jobs", "trash", "fileops", "undo"} {
			v1.Forget(e.deps, name)
		}
		shares.UserByID, shares.Notify = byID, notify
	})
	return e
}

func (e *env) real(api string) string {
	return filepath.Join(e.root, filepath.FromSlash(strings.TrimPrefix(api, "/mnt/user")))
}

func (e *env) write(api, content string) {
	e.t.Helper()
	p := e.real(api)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(api string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.real(api))
	if err != nil {
		e.t.Fatalf("read %s: %v", api, err)
	}
	return string(b)
}

type answer struct {
	code   int
	body   []byte
	header http.Header
}

func (a answer) into(t testing.TB, v any) {
	t.Helper()
	if err := json.Unmarshal(a.body, v); err != nil {
		t.Fatalf("%v in %s", err, a.body)
	}
}

// do sends a request. headers come in pairs, X-User logs in.
func (e *env) do(method, target string, body any, headers ...string) answer {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.srv.URL+target, rd)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return answer{resp.StatusCode, raw, resp.Header}
}

func (e *env) create(user string, req map[string]any) shares.Share {
	e.t.Helper()
	a := e.do("POST", "/shares", req, "X-User", user)
	if a.code != http.StatusCreated {
		e.t.Fatalf("create %v: %d %s", req, a.code, a.body)
	}
	var sh shares.Share
	a.into(e.t, &sh)
	return sh
}

func q(p string) string { return "?" + url.Values{"path": {p}}.Encode() }

func names(t *testing.T, a answer) []string {
	t.Helper()
	if a.code != http.StatusOK {
		t.Fatalf("list: %d %s", a.code, a.body)
	}
	var l shares.Listing
	a.into(t, &l)
	out := []string{}
	for _, it := range l.Items {
		out = append(out, it.Name)
	}
	return out
}

func TestCreatingALinkFillsItIn(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "limit": 5, "notify": true})
	if !regexp.MustCompile(`^[a-z0-9]{6}-urlaub$`).MatchString(sh.Slug) {
		t.Fatalf("slug %q", sh.Slug)
	}
	if sh.Kind != shares.KindFolder || sh.Name != "Urlaub" || sh.Owner != "anna" || sh.Limit != 5 || !sh.Notify || sh.Password || sh.Expires != nil || sh.State != shares.StateActive {
		t.Fatalf("link %+v", sh)
	}
	file := e.create("anna", map[string]any{"paths": []string{"/mnt/user/dokumente/rechnung.pdf"}, "slug": "Rechnung-Oktober", "password": "geheim1"})
	if file.Kind != shares.KindFile || file.Slug != "rechnung-oktober" || !file.Password {
		t.Fatalf("file link %+v", file)
	}

	var mine []shares.Share
	e.do("GET", "/shares", nil, "X-User", "anna").into(t, &mine)
	if len(mine) != 2 {
		t.Fatalf("own links %+v", mine)
	}
	if a := e.do("GET", "/shares", nil, "X-User", "bob"); string(a.body) != "[]\n" {
		t.Fatalf("bob sees %s", a.body)
	}
	var all []shares.Share
	e.do("GET", "/admin/shares", nil, "X-User", "jdp").into(t, &all)
	if len(all) != 2 {
		t.Fatalf("admin list %+v", all)
	}
	if a := e.do("GET", "/admin/shares", nil, "X-User", "anna"); a.code != http.StatusForbidden {
		t.Fatalf("admin list for anna: %d", a.code)
	}
	if strings.Contains(string(e.do("GET", "/shares/"+file.ID, nil, "X-User", "anna").body), "$2") {
		t.Fatal("the password hash leaks")
	}
}

func TestCreatingALinkChecksEverything(t *testing.T) {
	e := newEnv(t)
	e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "slug": "vergeben"})
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		user string
		req  map[string]any
		want int
	}{
		{"gast", map[string]any{"paths": []string{"/mnt/user/fotos"}}, http.StatusForbidden},
		{"eva", map[string]any{"paths": []string{"/mnt/user/fotos"}}, http.StatusForbidden},
		{"anna", map[string]any{"paths": []string{}}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/gibt-es-nicht"}}, http.StatusNotFound},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos/../privat"}}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt"}}, http.StatusForbidden},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos/x.zip::/a"}}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos/a", "/mnt/user/fotos/b"}}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/dokumente/rechnung.pdf"}, "uploads": true}, http.StatusBadRequest},
		{"bob", map[string]any{"paths": []string{"/mnt/user/eingang"}, "uploads": true}, http.StatusForbidden},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "slug": "ab"}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "slug": "-ab-c"}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "slug": "über-uns"}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "slug": "vergeben"}, http.StatusConflict},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "password": "kurz"}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "expires": past}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "limit": -1}, http.StatusBadRequest},
		{"anna", map[string]any{"paths": []string{"/mnt/user/fotos"}, "extra": 1}, http.StatusBadRequest},
	}
	for _, c := range cases {
		if a := e.do("POST", "/shares", c.req, "X-User", c.user); a.code != c.want {
			t.Errorf("%s %v: %d, want %d: %s", c.user, c.req, a.code, c.want, a.body)
		}
	}
}

func TestOnlyTheOwnerAndAdminsReachALink(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}})
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		if a := e.do(method, "/shares/"+sh.ID, map[string]any{}, "X-User", "bob"); a.code != http.StatusNotFound {
			t.Errorf("bob %s: %d", method, a.code)
		}
	}
	a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"limit": 3, "expires": time.Now().Add(48 * time.Hour)}, "X-User", "jdp")
	var got shares.Share
	a.into(t, &got)
	if a.code != http.StatusOK || got.Limit != 3 || got.Expires == nil || got.Owner != "anna" {
		t.Fatalf("admin edit: %d %s", a.code, a.body)
	}
	e.do("PATCH", "/shares/"+sh.ID, map[string]any{"expires": nil}, "X-User", "anna").into(t, &got)
	if got.Expires != nil || got.Limit != 3 {
		t.Fatalf("null expiry: %+v", got)
	}
	if a := e.do("DELETE", "/shares/"+sh.ID, nil, "X-User", "anna"); a.code != http.StatusNoContent {
		t.Fatalf("delete: %d", a.code)
	}
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusNotFound {
		t.Fatalf("a deleted link: %d", a.code)
	}
}

func TestVisitorsSeeTheFolderAndNothingAround(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}})
	var pub shares.Public
	a := e.do("GET", "/s/"+sh.Slug, nil)
	a.into(t, &pub)
	if a.code != http.StatusOK || pub.Kind != shares.KindFolder || pub.Name != "Urlaub" || pub.Locked || pub.Uploads {
		t.Fatalf("public view: %d %s", a.code, a.body)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list", nil)); strings.Join(got, ",") != "2025,strand.jpg" {
		t.Fatalf("root lists %v", got)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list"+q("/2025"), nil)); strings.Join(got, ",") != "meer.jpg" {
		t.Fatalf("subfolder lists %v", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q("/2025/meer.jpg"), nil); a.code != http.StatusOK || string(a.body) != "wellen" {
		t.Fatalf("raw: %d %s", a.code, a.body)
	}
	for p, want := range map[string]int{
		"/../privat/tagebuch.txt": http.StatusBadRequest,
		"/2025/../../privat":      http.StatusBadRequest,
		"/.geheim":                http.StatusNotFound,
		"/strand.jpg::/x":         http.StatusBadRequest,
		"/fehlt.jpg":              http.StatusNotFound,
		"%2e%2e/privat":           http.StatusNotFound,
	} {
		if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q(p), nil); a.code != want {
			t.Errorf("raw %s: %d, want %d", p, a.code, want)
		}
	}
	for _, body := range [][]byte{e.do("GET", "/s/"+sh.Slug+"/list", nil).body, e.do("GET", "/s/"+sh.Slug, nil).body} {
		if strings.Contains(string(body), "/mnt/user") || strings.Contains(string(body), "anna") {
			t.Fatalf("the visitor learns too much: %s", body)
		}
	}
}

func TestLinksInsideASharedFolderCannotLeadOut(t *testing.T) {
	e := newEnv(t)
	if err := os.Symlink(e.real("/mnt/user/privat"), e.real("/mnt/user/fotos/Urlaub/raus")); err != nil {
		t.Skip("no linked folders here:", err)
	}
	if err := os.Symlink(e.real("/mnt/user/privat/tagebuch.txt"), e.real("/mnt/user/fotos/Urlaub/tagebuch.txt")); err != nil {
		t.Fatal(err)
	}
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "uploads": true})
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list", nil)); strings.Join(got, ",") != "2025,strand.jpg" {
		t.Fatalf("links out are listed: %v", got)
	}
	for _, p := range []string{"/raus/tagebuch.txt", "/tagebuch.txt"} {
		if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q(p), nil); a.code != http.StatusNotFound {
			t.Errorf("raw %s through a link: %d %s", p, a.code, a.body)
		}
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/list"+q("/raus"), nil); a.code != http.StatusNotFound {
		t.Errorf("list through a link: %d", a.code)
	}
	if a := e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "raus/boese.txt", "size": 1}); a.code != http.StatusNotFound {
		t.Errorf("upload through a link: %d %s", a.code, a.body)
	}
	zr := unzip(t, e.do("GET", "/s/"+sh.Slug+"/raw", nil))
	for _, f := range zr {
		if strings.Contains(f, "tagebuch") || strings.Contains(f, "raus") || strings.Contains(f, "geheim") {
			t.Fatalf("the zip holds %v", zr)
		}
	}
}

func TestAPasswordLocksTheLink(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "password": "sonnenschein"})
	a := e.do("GET", "/s/"+sh.Slug, nil)
	if a.code != http.StatusOK || string(a.body) != `{"slug":"`+sh.Slug+`","locked":true}`+"\n" {
		t.Fatalf("locked view: %d %s", a.code, a.body)
	}
	for _, target := range []string{"/list", "/raw" + q("/strand.jpg")} {
		if a := e.do("GET", "/s/"+sh.Slug+target, nil); a.code != http.StatusUnauthorized {
			t.Errorf("%s while locked: %d", target, a.code)
		}
	}
	if a := e.do("POST", "/s/"+sh.Slug+"/unlock", map[string]string{"password": "regen"}); a.code != http.StatusForbidden {
		t.Fatalf("wrong password: %d", a.code)
	}
	a = e.do("POST", "/s/"+sh.Slug+"/unlock", map[string]string{"password": "sonnenschein"})
	var un shares.Unlocked
	a.into(t, &un)
	if a.code != http.StatusOK || un.Token == "" || !un.Expires.After(time.Now()) {
		t.Fatalf("unlock: %d %s", a.code, a.body)
	}
	cookie := a.header.Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") {
		t.Fatalf("cookie %q", cookie)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list", nil, "X-Share-Token", un.Token)); len(got) != 2 {
		t.Fatalf("list with the token: %v", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw?path=/strand.jpg&token="+url.QueryEscape(un.Token), nil); a.code != http.StatusOK {
		t.Fatalf("raw with the token: %d", a.code)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/list", nil, "Cookie", strings.Split(cookie, ";")[0]); a.code != http.StatusOK {
		t.Fatalf("list with the cookie: %d", a.code)
	}
	for _, bad := range []string{"x", un.Token + "x", "AAAAAAAAAAA." + strings.Split(un.Token, ".")[1]} {
		if a := e.do("GET", "/s/"+sh.Slug+"/list", nil, "X-Share-Token", bad); a.code != http.StatusUnauthorized {
			t.Errorf("forged token %q: %d", bad, a.code)
		}
	}

	other := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "password": "sonnenschein"})
	if a := e.do("GET", "/s/"+other.Slug+"/list", nil, "X-Share-Token", un.Token); a.code != http.StatusUnauthorized {
		t.Fatalf("the token of one link opens another: %d", a.code)
	}

	e.do("PATCH", "/shares/"+sh.ID, map[string]any{"password": "neuespasswort"}, "X-User", "anna")
	if a := e.do("GET", "/s/"+sh.Slug+"/list", nil, "X-Share-Token", un.Token); a.code != http.StatusUnauthorized {
		t.Fatalf("a new password keeps old browsers in: %d", a.code)
	}
	e.do("PATCH", "/shares/"+sh.ID, map[string]any{"password": ""}, "X-User", "anna")
	if a := e.do("GET", "/s/"+sh.Slug+"/list", nil); a.code != http.StatusOK {
		t.Fatalf("a link without password: %d", a.code)
	}
}

func TestGuessingPasswordsHasToWait(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "password": "sonnenschein"})
	for i := range 5 {
		if a := e.do("POST", "/s/"+sh.Slug+"/unlock", map[string]string{"password": "falsch" + strconv.Itoa(i)}); a.code != http.StatusForbidden {
			t.Fatalf("try %d: %d", i, a.code)
		}
	}
	a := e.do("POST", "/s/"+sh.Slug+"/unlock", map[string]string{"password": "sonnenschein"})
	if a.code != http.StatusTooManyRequests || a.header.Get("Retry-After") == "" {
		t.Fatalf("after five wrong passwords: %d %v", a.code, a.header)
	}
}

func TestGuessingAddressesHasToWait(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}})
	code := 0
	for i := 0; i < 40 && code != http.StatusTooManyRequests; i++ {
		code = e.do("GET", "/s/raten-"+strconv.Itoa(i), nil).code
	}
	if code != http.StatusTooManyRequests {
		t.Fatalf("40 unknown addresses: %d", code)
	}
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusTooManyRequests {
		t.Fatalf("a guesser still reaches links: %d", a.code)
	}
}

func TestExpiredLinksLeadNowhere(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "expires": time.Now().Add(1100 * time.Millisecond)})
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusOK {
		t.Fatalf("before expiry: %d", a.code)
	}
	time.Sleep(2 * time.Second)
	for _, target := range []string{"", "/list", "/raw" + q("/IMG.jpg")} {
		if a := e.do("GET", "/s/"+sh.Slug+target, nil); a.code != http.StatusGone {
			t.Errorf("%s after expiry: %d", target, a.code)
		}
	}
	var got shares.Share
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.State != shares.StateExpired {
		t.Fatalf("state %s", got.State)
	}
	a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"expires": time.Now().Add(24 * time.Hour)}, "X-User", "anna")
	if a.code != http.StatusOK || e.do("GET", "/s/"+sh.Slug, nil).code != http.StatusOK {
		t.Fatalf("an extended link: %d %s", a.code, a.body)
	}
}

func TestTheDownloadLimitCountsDownloadsOnly(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/dokumente/rechnung.pdf"}, "limit": 2})
	raw := "/s/" + sh.Slug + "/raw"
	for range 3 {
		if a := e.do("GET", raw, nil); a.code != http.StatusOK || string(a.body) != "%PDF rechnung" {
			t.Fatalf("view: %d", a.code)
		}
	}
	if a := e.do("GET", raw+"?download=1", nil); a.code != http.StatusOK || !strings.Contains(a.header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("download: %d %v", a.code, a.header)
	}
	if a := e.do("GET", raw+"?download=1", nil, "Range", "bytes=5-"); a.code != http.StatusPartialContent || string(a.body) != "rechnung" {
		t.Fatalf("resumed download: %d %s", a.code, a.body)
	}
	if a := e.do("GET", raw+q("/rechnung.pdf")+"&download=1", nil); a.code != http.StatusOK {
		t.Fatalf("second download: %d", a.code)
	}
	if a := e.do("GET", raw+"?download=1", nil); a.code != http.StatusGone {
		t.Fatalf("third download: %d", a.code)
	}
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusGone {
		t.Fatalf("a used up link opens: %d", a.code)
	}
	var got shares.Share
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.State != shares.StateExhausted || got.Downloads != 2 {
		t.Fatalf("link %+v", got)
	}
}

func TestAFileLinkShowsOnlyItsFile(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/dokumente/rechnung.pdf"}})
	var pub shares.Public
	e.do("GET", "/s/"+sh.Slug, nil).into(t, &pub)
	if pub.Kind != shares.KindFile || pub.Name != "rechnung.pdf" || pub.Size != 13 || pub.MTime == nil {
		t.Fatalf("public view %+v", pub)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list", nil)); strings.Join(got, ",") != "rechnung.pdf" {
		t.Fatalf("list %v", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q("/andere.pdf"), nil); a.code != http.StatusNotFound {
		t.Fatalf("another file: %d", a.code)
	}
	if a := e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "x.txt", "size": 1}); a.code != http.StatusForbidden {
		t.Fatalf("upload into a file link: %d", a.code)
	}
}

func unzip(t *testing.T, a answer) []string {
	t.Helper()
	if a.code != http.StatusOK {
		t.Fatalf("archive: %d %s", a.code, a.body)
	}
	zr, err := zip.NewReader(bytes.NewReader(a.body), int64(len(a.body)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	return out
}

func TestACollectionLooksLikeOneFolder(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"name": "Sammlung vom 04.11", "limit": 1,
		"paths": []string{"/mnt/user/fotos/a/IMG.jpg", "/mnt/user/fotos/b/IMG.jpg", "/mnt/user/fotos/Urlaub"}})
	if sh.Kind != shares.KindCollection || sh.Name != "Sammlung vom 04.11" || len(sh.Paths) != 3 {
		t.Fatalf("collection %+v", sh)
	}
	var pub shares.Public
	e.do("GET", "/s/"+sh.Slug, nil).into(t, &pub)
	if pub.Count != 3 || pub.Name != "Sammlung vom 04.11" {
		t.Fatalf("public view %+v", pub)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list", nil)); strings.Join(got, ",") != "IMG.jpg,IMG (2).jpg,Urlaub" {
		t.Fatalf("root %v", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q("/IMG (2).jpg"), nil); string(a.body) != "zwei" {
		t.Fatalf("second IMG: %d %s", a.code, a.body)
	}
	if got := names(t, e.do("GET", "/s/"+sh.Slug+"/list"+q("/Urlaub/2025"), nil)); strings.Join(got, ",") != "meer.jpg" {
		t.Fatalf("inside an entry %v", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw"+q("/a/IMG.jpg"), nil); a.code != http.StatusNotFound {
		t.Fatalf("a path the collection does not have: %d", a.code)
	}
	a := e.do("GET", "/s/"+sh.Slug+"/raw", nil)
	if !strings.Contains(a.header.Get("Content-Disposition"), "Sammlung vom 04.11.zip") {
		t.Fatalf("disposition %q", a.header.Get("Content-Disposition"))
	}
	got := strings.Join(unzip(t, a), ",")
	if !strings.Contains(got, "IMG (2).jpg") || !strings.Contains(got, "Urlaub/2025/meer.jpg") || strings.Contains(got, ".geheim") {
		t.Fatalf("zip %s", got)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/raw", nil); a.code != http.StatusGone {
		t.Fatalf("the zip does not count as a download: %d", a.code)
	}
}

func TestOpeningALinkTellsTheOwnerOnce(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "notify": true})
	c, _, _ := e.deps.Hub.Connect("tab-1", "anna", 0)
	other, _, _ := e.deps.Hub.Connect("tab-2", "bob", 0)
	e.do("GET", "/s/"+sh.Slug, nil)
	e.do("GET", "/s/"+sh.Slug, nil)
	select {
	case ev := <-c.Events:
		if ev.Type != shares.EventOpened || !strings.Contains(string(ev.Data), `"views":1`) || !strings.Contains(string(ev.Data), sh.Slug) {
			t.Fatalf("event %s %s", ev.Type, ev.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no share.opened")
	}
	select {
	case ev := <-c.Events:
		t.Fatalf("a second event %s", ev.Type)
	case ev := <-other.Events:
		t.Fatalf("someone else got %s", ev.Type)
	case <-time.After(100 * time.Millisecond):
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.notices) != 1 || !strings.Contains(e.notices[0].Description, "Urlaub") {
		t.Fatalf("notices %+v", e.notices)
	}
	var got shares.Share
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.Views != 1 {
		t.Fatalf("views %d", got.Views)
	}
}

func (e *env) dropFile(slug string, body map[string]any, content string) fileops.Upload {
	e.t.Helper()
	a := e.do("POST", "/s/"+slug+"/upload", body)
	if a.code != http.StatusCreated {
		e.t.Fatalf("start %v: %d %s", body, a.code, a.body)
	}
	var up fileops.Upload
	a.into(e.t, &up)
	a = e.do("PUT", "/s/"+slug+"/upload/"+up.ID, content, "Upload-Offset", "0")
	if a.code != http.StatusOK {
		e.t.Fatalf("chunk: %d %s", a.code, a.body)
	}
	a.into(e.t, &up)
	if strings.Contains(string(a.body), "/mnt/user") {
		e.t.Fatalf("the visitor learns the server path: %s", a.body)
	}
	return up
}

func TestVisitorsDropFilesIntoALink(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/eingang"}, "uploads": true})
	var pub shares.Public
	e.do("GET", "/s/"+sh.Slug, nil).into(t, &pub)
	if !pub.Uploads || pub.UploadRoom != nil {
		t.Fatalf("public view %+v", pub)
	}
	up := e.dropFile(sh.Slug, map[string]any{"name": "brief.txt", "size": 5}, "hallo")
	if up.State != fileops.UploadDone || up.Path != "/brief.txt" || e.read("/mnt/user/eingang/brief.txt") != "hallo" {
		t.Fatalf("first upload %+v", up)
	}
	up = e.dropFile(sh.Slug, map[string]any{"name": "brief.txt", "size": 4}, "neu!")
	if up.Path != "/brief (1).txt" || e.read("/mnt/user/eingang/brief.txt") != "hallo" {
		t.Fatalf("a second file of the same name %+v", up)
	}
	up = e.dropFile(sh.Slug, map[string]any{"name": "Urlaub/2025/bild.jpg", "size": 3}, "jpg")
	if up.Path != "/Urlaub/2025/bild.jpg" || e.read("/mnt/user/eingang/Urlaub/2025/bild.jpg") != "jpg" {
		t.Fatalf("a dropped folder %+v", up)
	}
	var got shares.Share
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.Uploaded != 12 {
		t.Fatalf("uploaded %d", got.Uploaded)
	}
	for _, body := range []map[string]any{
		{"name": "brief.txt", "size": 1, "rule": "replace"},
		{"name": "brief.txt", "size": 1, "rule": "ask"},
		{"name": ".htaccess", "size": 1},
		{"name": "a/.git/config", "size": 1},
		{"name": "../privat/x.txt", "size": 1},
		{"name": "", "size": 1},
		{"name": "x.txt", "size": -1},
		{"name": "x.txt", "size": 1, "path": "/../privat"},
	} {
		if a := e.do("POST", "/s/"+sh.Slug+"/upload", body); a.code != http.StatusBadRequest {
			t.Errorf("%v: %d %s", body, a.code, a.body)
		}
	}
	plain := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}})
	if a := e.do("POST", "/s/"+plain.Slug+"/upload", map[string]any{"name": "x.txt", "size": 1}); a.code != http.StatusForbidden {
		t.Fatalf("upload into a link without uploads: %d", a.code)
	}
}

func TestTwoVisitorsUploadingTheSameNameKeepBothFiles(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/eingang"}, "uploads": true})
	var first, second fileops.Upload
	e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "foto.jpg", "size": 3}).into(t, &first)
	e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "foto.jpg", "size": 3}).into(t, &second)
	if first.ID == second.ID || second.Path != "/foto (1).jpg" {
		t.Fatalf("second upload %+v", second)
	}
	e.do("PUT", "/s/"+sh.Slug+"/upload/"+second.ID, "bbb", "Upload-Offset", "0")
	e.do("PUT", "/s/"+sh.Slug+"/upload/"+first.ID, "aaa", "Upload-Offset", "0")
	if e.read("/mnt/user/eingang/foto.jpg") != "aaa" || e.read("/mnt/user/eingang/foto (1).jpg") != "bbb" {
		t.Fatal("the uploads got mixed up")
	}
}

func TestTheUploadLimitHoldsAlsoForRunningUploads(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/eingang"}, "uploads": true, "uploadLimit": 10})
	e.dropFile(sh.Slug, map[string]any{"name": "a.txt", "size": 4}, "1234")
	var open fileops.Upload
	if a := e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "b.txt", "size": 4}); a.code != http.StatusCreated {
		t.Fatalf("second upload: %d", a.code)
	} else {
		a.into(t, &open)
	}
	if a := e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "c.txt", "size": 3}); a.code != http.StatusRequestEntityTooLarge {
		t.Fatalf("past the limit: %d %s", a.code, a.body)
	}
	if a := e.do("PUT", "/s/"+sh.Slug+"/upload/"+open.ID, "12345", "Upload-Offset", "0"); a.code != http.StatusRequestEntityTooLarge {
		t.Fatalf("more bytes than announced: %d", a.code)
	}
	if a := e.do("DELETE", "/s/"+sh.Slug+"/upload/"+open.ID, nil); a.code != http.StatusNoContent {
		t.Fatalf("cancel: %d", a.code)
	}
	var pub shares.Public
	e.do("GET", "/s/"+sh.Slug, nil).into(t, &pub)
	if pub.UploadRoom == nil || *pub.UploadRoom != 6 {
		t.Fatalf("room %+v", pub.UploadRoom)
	}
	e.dropFile(sh.Slug, map[string]any{"name": "c.txt", "size": 6}, "123456")
}

func TestVisitorsOnlyReachUploadsOfTheirLink(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/eingang"}, "uploads": true})
	other := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/a"}, "uploads": true})
	var up fileops.Upload
	e.do("POST", "/s/"+sh.Slug+"/upload", map[string]any{"name": "a.txt", "size": 2}).into(t, &up)
	if a := e.do("GET", "/s/"+other.Slug+"/upload/"+up.ID, nil); a.code != http.StatusNotFound {
		t.Fatalf("an upload through another link: %d", a.code)
	}
	if a := e.do("GET", "/uploads/"+up.ID, nil, "X-User", "anna"); a.code != http.StatusNotFound {
		t.Fatalf("a share upload among the owner's own: %d", a.code)
	}
	if a := e.do("GET", "/s/"+sh.Slug+"/upload/"+up.ID, nil); a.code != http.StatusOK || !strings.Contains(string(a.body), `"path":"/a.txt"`) {
		t.Fatalf("state: %d %s", a.code, a.body)
	}
	e.do("DELETE", "/shares/"+sh.ID, nil, "X-User", "anna")
	if _, err := os.Stat(e.real("/mnt/user/eingang/.a.txt.cc-part")); !os.IsNotExist(err) {
		t.Fatal("deleting the link left its part file")
	}
}

func TestALinkEndsWithItsOwnersRights(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}})
	e.people["anna"].Permissions.Share = false
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusGone {
		t.Fatalf("owner without the share right: %d", a.code)
	}
	e.people["anna"].Permissions.Share = true
	e.people["anna"].BackendScopes[0].Scope = "/dokumente"
	if a := e.do("GET", "/s/"+sh.Slug+"/list", nil); a.code != http.StatusForbidden {
		t.Fatalf("owner without access to the folder: %d", a.code)
	}
	delete(e.people, "anna")
	if a := e.do("GET", "/s/"+sh.Slug, nil); a.code != http.StatusGone {
		t.Fatalf("owner gone: %d", a.code)
	}
}

func TestLinksFollowRenamesAndGetNewAddresses(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}})
	col := e.create("anna", map[string]any{"name": "Mix", "paths": []string{"/mnt/user/fotos/Urlaub/strand.jpg", "/mnt/user/fotos/a/IMG.jpg"}})
	fschange.Moved("/mnt/user/fotos", "/mnt/user/bilder")
	var got shares.Share
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.Paths[0] != "/mnt/user/bilder/Urlaub" {
		t.Fatalf("paths %v", got.Paths)
	}
	e.do("GET", "/shares/"+col.ID, nil, "X-User", "anna").into(t, &got)
	if got.Paths[0] != "/mnt/user/bilder/Urlaub/strand.jpg" || got.Paths[1] != "/mnt/user/bilder/a/IMG.jpg" {
		t.Fatalf("collection paths %v", got.Paths)
	}
	fschange.Moved("/mnt/user/bild", "/mnt/user/x")
	e.do("GET", "/shares/"+sh.ID, nil, "X-User", "anna").into(t, &got)
	if got.Paths[0] != "/mnt/user/bilder/Urlaub" {
		t.Fatalf("a rename of a sibling with the same start moved the link: %v", got.Paths)
	}

	a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"reroll": true}, "X-User", "anna")
	a.into(t, &got)
	if a.code != http.StatusOK || got.Slug == sh.Slug || !strings.HasSuffix(got.Slug, "-urlaub") {
		t.Fatalf("reroll: %d %s", a.code, a.body)
	}
	if e.do("GET", "/s/"+sh.Slug, nil).code != http.StatusNotFound {
		t.Fatal("the old address still works")
	}
	if a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"slug": col.Slug}, "X-User", "anna"); a.code != http.StatusConflict {
		t.Fatalf("a taken address: %d", a.code)
	}
	if a := e.do("PATCH", "/shares/"+sh.ID, map[string]any{"slug": "urlaub-2026"}, "X-User", "anna"); a.code != http.StatusOK {
		t.Fatalf("an own address: %d %s", a.code, a.body)
	}
}

func TestRemovingAUserRemovesTheirLinks(t *testing.T) {
	e := newEnv(t)
	e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}})
	keep := e.create("jdp", map[string]any{"paths": []string{"/mnt/user/fotos/a"}})
	if err := shares.RemoveUser(t.Context(), e.deps, e.people["anna"].ID); err != nil {
		t.Fatal(err)
	}
	var all []shares.Share
	e.do("GET", "/admin/shares", nil, "X-User", "jdp").into(t, &all)
	if len(all) != 1 || all[0].ID != keep.ID {
		t.Fatalf("left %+v", all)
	}
}

func TestALinkComesWithItsAddressAsAQRCode(t *testing.T) {
	e := newEnv(t)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}, "slug": "urlaub-2026"})
	var got shares.Address
	a := e.do("GET", "/shares/"+sh.ID+"/qr?origin="+url.QueryEscape("https://files.example.org/"), nil, "X-User", "anna")
	a.into(t, &got)
	if a.code != http.StatusOK || got.URL != "https://files.example.org/s/urlaub-2026" || !strings.HasPrefix(got.QR, "<svg") {
		t.Fatalf("address %d %+v", a.code, got)
	}
	e.do("GET", "/shares/"+sh.ID+"/qr", nil, "X-User", "anna", "X-Forwarded-Proto", "https", "X-Forwarded-Host", "cannon.lan").into(t, &got)
	if got.URL != "https://cannon.lan/s/urlaub-2026" {
		t.Fatalf("forwarded address %q", got.URL)
	}
	for _, bad := range []string{"javascript:alert(1)", "https://files.example.org/elsewhere", "https://user@files.example.org"} {
		if a := e.do("GET", "/shares/"+sh.ID+"/qr?origin="+url.QueryEscape(bad), nil, "X-User", "anna"); a.code != http.StatusBadRequest {
			t.Fatalf("origin %q: %d", bad, a.code)
		}
	}
	if a := e.do("GET", "/shares/"+sh.ID+"/qr", nil, "X-User", "bob"); a.code != http.StatusNotFound {
		t.Fatalf("bob reads the address: %d", a.code)
	}
}
