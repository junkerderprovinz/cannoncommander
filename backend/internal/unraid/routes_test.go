package unraid

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// routesFixture serves the package over HTTP. The user share is a plain
// folder here, so files that matter through /mnt/user exist there as well
// as on their disk, the way shfs shows them.
func routesFixture(t *testing.T) (*env, http.Handler) {
	t.Helper()
	e := newEnv(t)
	for _, f := range []struct {
		dev, rel string
	}{{"disk2", "filme/Sintel/Sintel.mkv"}, {"disk3", "fotos/2025/a.jpg"}} {
		e.file(f.dev, f.rel, 2)
		e.file("user", f.rel, 2)
	}
	e.dir("user", "appdata")
	user := filepath.Join(e.cfg.Mnt, "user")
	r, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: user},
		ccfs.Source{Name: "Disk 2", Mount: "/mnt/disk2", Root: filepath.Join(e.cfg.Mnt, "disk2"), AdminOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	people := map[string]*users.User{
		"jdp":  {BackendScopes: []users.BackendScope{{Path: user, Scope: "/", Permissions: all}, {Path: filepath.Join(e.cfg.Mnt, "disk2"), Scope: "/", Permissions: all}}},
		"anna": {BackendScopes: []users.BackendScope{{Path: user, Scope: "/fotos", Permissions: all}}},
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
	router := v1.NewRouter(v1.Deps{FS: r}, auth)
	router.Contract("unraid", contract)
	bind(router, e.svc)
	h, err := router.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return e, h
}

func call(t *testing.T, h http.Handler, who, method, target, body string, out any) int {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("X-User", who)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v in %s", method, target, err, rec.Body)
		}
	}
	return rec.Code
}

func TestStateEndpoint(t *testing.T) {
	_, h := routesFixture(t)
	var st State
	if code := call(t, h, "anna", "GET", "/unraid/state", "", &st); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if !st.Available || st.Version != "7.3.4" || len(st.Disks) != 4 || len(st.Shares) != len(fakeShares) || st.Write {
		t.Errorf("state = %+v", st)
	}
}

func TestWhereEndpointChecksAccess(t *testing.T) {
	_, h := routesFixture(t)
	var w Where
	if code := call(t, h, "jdp", "GET", "/unraid/where?path="+url.QueryEscape("/mnt/user/filme/Sintel"), "", &w); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if !w.Dir || len(w.On) != 1 || w.On[0].Device != "disk2" {
		t.Errorf("where = %+v", w)
	}
	if code := call(t, h, "anna", "GET", "/unraid/where?path=/mnt/user/filme", "", nil); code != http.StatusForbidden {
		t.Errorf("anna outside her scope: %d", code)
	}
	if code := call(t, h, "anna", "GET", "/unraid/where?path=/mnt/user/fotos/2025/a.jpg", "", &w); code != http.StatusOK || w.Dir || w.On[0].Device != "disk3" {
		t.Errorf("anna in her scope: %d %+v", code, w)
	}
}

func TestTransferEndpointReportsTheTrap(t *testing.T) {
	_, h := routesFixture(t)
	var rep TransferReport
	body := `{"kind":"copy","paths":["/mnt/disk2/filme/Sintel"],"dest":"/mnt/user/filme"}`
	if code := call(t, h, "jdp", "POST", "/unraid/transfer", body, &rep); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if rep.Trap == nil || rep.Trap.Count != 1 || rep.Trap.Files[0].Path != "/mnt/user/filme/Sintel/Sintel.mkv" {
		t.Errorf("trap = %+v", rep.Trap)
	}
	if code := call(t, h, "anna", "POST", "/unraid/transfer", body, nil); code != http.StatusForbidden {
		t.Errorf("anna may not see disk2: %d", code)
	}
	if code := call(t, h, "jdp", "POST", "/unraid/transfer", `{"kind":"sync","paths":["/mnt/user/filme"],"dest":"/mnt/user"}`, nil); code != http.StatusBadRequest {
		t.Errorf("unknown kind: %d", code)
	}
}

func TestAdminEndpoints(t *testing.T) {
	_, h := routesFixture(t)
	for _, c := range []struct {
		method, target, body string
		anna, jdp            int
	}{
		{"POST", "/unraid/spindown", "", http.StatusForbidden, http.StatusForbidden},
		{"POST", "/unraid/notify/test", "", http.StatusForbidden, http.StatusForbidden},
		{"POST", "/unraid/gather/plan", `{"path":"/mnt/user/filme","mode":"gather"}`, http.StatusForbidden, http.StatusOK},
		{"POST", "/unraid/gather/plan", `{"path":"/mnt/user/filme","mode":"stack"}`, http.StatusForbidden, http.StatusBadRequest},
		{"POST", "/unraid/fixperms/scan", `{"path":"/mnt/user/appdata"}`, http.StatusForbidden, http.StatusOK},
	} {
		if code := call(t, h, "anna", c.method, c.target, c.body, nil); code != c.anna {
			t.Errorf("anna %s %s: %d, want %d", c.target, c.body, code, c.anna)
		}
		if code := call(t, h, "jdp", c.method, c.target, c.body, nil); code != c.jdp {
			t.Errorf("jdp %s %s: %d, want %d", c.target, c.body, code, c.jdp)
		}
	}
	var scan PermsScan
	call(t, h, "jdp", "POST", "/unraid/fixperms/scan", `{"path":"/mnt/user/appdata"}`, &scan)
	if scan.Blocked != KeepAppdata {
		t.Errorf("scan = %+v", scan)
	}
}
