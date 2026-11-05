package dav_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/dav"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

var (
	everything = users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	reader     = users.SourceFilePermissions{View: true, Download: true}
)

// rules hides or locks paths for everybody except admins. A key without a
// leading slash is a name, hidden wherever it is.
type rules map[string]ccfs.Effect

func (r rules) Effect(u *users.User, p string) ccfs.Effect {
	if u.Permissions.Admin {
		return ccfs.Open
	}
	if effect, ok := r[path.Base(p)]; ok {
		return effect
	}
	return r.onPath(p)
}

func (r rules) onPath(p string) ccfs.Effect {
	for at, effect := range r {
		if p == at || strings.HasPrefix(p, at+"/") {
			return effect
		}
	}
	return ccfs.Open
}

func (r rules) CarryEffect(u *users.User, p string) ccfs.Effect {
	if u.Permissions.Admin {
		return ccfs.Open
	}
	return r.onPath(p)
}

func (r rules) CarryBelow(u *users.User, _ string) bool {
	if u.Permissions.Admin {
		return false
	}
	for at := range r {
		if strings.HasPrefix(at, "/") {
			return true
		}
	}
	return false
}

type refreshLog struct {
	fsindex.Index
	mu   sync.Mutex
	dirs []string
}

func (l *refreshLog) Refresh(_ *ccfs.Source, dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dirs = append(l.dirs, dir)
}

// env serves /dav/ over the share root at /mnt/user, the read only source
// at /mnt/usb and the disk at /mnt/disk1. The name of a request picks the
// rights, a password of "r" stands for a read only token, "wrong" for a
// failed sign in and "locked" for a client that failed too often.
type env struct {
	t       *testing.T
	root    string
	usb     string
	disk    string
	on      bool
	rules   rules
	deps    v1.Deps
	index   *refreshLog
	h       *dav.Handler
	srv     *httptest.Server
	signIns int
}

func newEnv(t *testing.T) *env { return newEnvOn(t, t.TempDir()) }

func newEnvOn(t *testing.T, disk string) *env {
	t.Helper()
	e := &env{t: t, root: t.TempDir(), usb: t.TempDir(), disk: disk, on: true, rules: rules{}, index: &refreshLog{}}
	fsys, err := ccfs.New(e.rules,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: e.root},
		ccfs.Source{Name: "USB", Mount: "/mnt/usb", Root: e.usb, ReadOnly: true},
		ccfs.Source{Name: "Disk", Mount: "/mnt/disk1", Root: e.disk},
	)
	if err != nil {
		t.Fatal(err)
	}
	e.deps = v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t), Index: e.index}
	e.h = dav.New(e.deps, dav.Options{Prefix: "/dav", Enabled: func() bool { return e.on }, SignIn: e.signIn})
	mux := http.NewServeMux()
	mux.Handle("/dav", e.h)
	mux.Handle("/dav/", e.h)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	e.write("fotos/a.jpg", "sonne")
	return e
}

func (e *env) signIn(_ http.ResponseWriter, r *http.Request) (dav.Identity, error) {
	e.signIns++
	name, password, ok := r.BasicAuth()
	switch {
	case !ok || password == "wrong":
		return dav.Identity{}, errors.New("refused")
	case password == "locked":
		return dav.Identity{}, &accounts.TooManyTries{Wait: 90 * time.Second}
	}
	u := e.user(name)
	id := dav.Identity{User: u}
	if password == "r" {
		u.RestrictToReading()
		id.ReadOnly = true
	}
	return id, nil
}

func (e *env) user(name string) *users.User {
	perms := everything
	scope := "/"
	switch name {
	case "gast":
		perms = reader
	case "seher":
		perms = users.SourceFilePermissions{View: true}
	case "neuling":
		perms = users.SourceFilePermissions{View: true, Download: true, Create: true}
	case "bearbeiter":
		perms = users.SourceFilePermissions{View: true, Download: true, Modify: true}
	case "loescher":
		perms = users.SourceFilePermissions{View: true, Download: true, Delete: true}
	case "bea":
		scope = "/fotos"
	}
	u := &users.User{}
	u.Username = name
	u.Permissions.Admin = name == "admin"
	for _, root := range []string{e.root, e.usb, e.disk} {
		u.BackendScopes = append(u.BackendScopes, users.BackendScope{Path: root, Scope: scope, Permissions: perms})
	}
	return u
}

// real maps a path below /mnt/user to the disk.
func (e *env) real(p string) string { return filepath.Join(e.root, filepath.FromSlash(p)) }

func (e *env) write(p, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.real(p)), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(e.real(p), []byte(content), 0o666); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(p string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.real(p))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

func (e *env) exists(p string) bool {
	_, err := os.Lstat(e.real(p))
	return err == nil
}

type answer struct {
	status int
	header http.Header
	body   string
}

type hdr map[string]string

func (e *env) ask(user, method, p, body string, headers hdr) answer {
	e.t.Helper()
	return e.askAs(user, "pw", method, p, body, headers)
}

func (e *env) askAs(user, password, method, p, body string, headers hdr) answer {
	e.t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+"/dav"+p, strings.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	if user != "" {
		req.SetBasicAuth(user, password)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := e.srv.Client().Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return answer{res.StatusCode, res.Header, string(b)}
}

func (e *env) dest(p string) hdr { return hdr{"Destination": e.srv.URL + "/dav" + p, "Overwrite": "F"} }

func (e *env) over(p string) hdr { return hdr{"Destination": e.srv.URL + "/dav" + p, "Overwrite": "T"} }

func (e *env) want(a answer, status int) {
	e.t.Helper()
	if a.status != status {
		e.t.Fatalf("status %d, want %d: %s", a.status, status, a.body)
	}
}

var hrefs = regexp.MustCompile(`<D:href>([^<]*)</D:href>`)

// list returns the names PROPFIND shows inside the folder p.
func (e *env) list(user, p string) []string {
	e.t.Helper()
	a := e.ask(user, "PROPFIND", p, "", hdr{"Depth": "1"})
	e.want(a, http.StatusMultiStatus)
	self := strings.TrimSuffix("/dav"+p, "/")
	names := []string{}
	for _, m := range hrefs.FindAllStringSubmatch(a.body, -1) {
		href := strings.TrimSuffix(m[1], "/")
		if href != self {
			names = append(names, path.Base(href))
		}
	}
	slices.Sort(names)
	return names
}

func eq(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// symlink adds a link at p below /mnt/user, or skips where links need rights.
func (e *env) symlink(target, p string) {
	e.t.Helper()
	if err := os.Symlink(target, e.real(p)); err != nil {
		e.t.Skipf("no symlinks here: %v", err)
	}
}

func (e *env) label(p, colour string) {
	e.t.Helper()
	if _, err := e.deps.DB.Exec(`INSERT INTO label_items (path, dir, colour, updated_at) VALUES (?, ?, ?, 0)`, p, path.Dir(p), colour); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) colour(p string) string {
	e.t.Helper()
	var c string
	if err := e.deps.DB.QueryRow(`SELECT COALESCE(colour, '') FROM label_items WHERE path = ?`, p).Scan(&c); err != nil {
		return ""
	}
	return c
}
