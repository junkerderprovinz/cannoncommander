package remotes

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

const fakeProviders = `{"providers":[
 {"Name":"sftp","Description":"SSH/SFTP","Options":[
  {"Name":"host","Required":true,"Help":"SSH host to connect to."},
  {"Name":"user","Help":"SSH username."},
  {"Name":"port","DefaultStr":"22","Help":"SSH port number."},
  {"Name":"pass","IsPassword":true,"Help":"SSH password."},
  {"Name":"key_file","Advanced":true,"Help":"Path to PEM-encoded private key file."}]},
 {"Name":"smb","Description":"SMB / CIFS","Options":[
  {"Name":"host","Required":true},{"Name":"user"},{"Name":"pass","IsPassword":true},{"Name":"domain","DefaultStr":"WORKGROUP"}]},
 {"Name":"drive","Description":"Google Drive","Options":[
  {"Name":"client_id"},{"Name":"client_secret"},{"Name":"token","Advanced":true},{"Name":"scope"}]},
 {"Name":"memory","Description":"In memory object storage system.","Options":[]},
 {"Name":"tardigrade","Description":"Storj (old name)","Hide":true,"Options":[]}
]}`

// fakeRC answers like rclone rcd with a config in memory. Google Drive asks
// its questions after the token the way rclone does.
type fakeRC struct {
	t *testing.T

	mu            sync.Mutex
	sections      map[string]map[string]string
	mounts        map[string]string
	vfs           map[string]map[string]any
	opts          map[string]map[string]any
	calls         []string
	unmountRefuse bool
}

func newFakeRC(t *testing.T) (*fakeRC, *rcClient) {
	f := &fakeRC{t: t, sections: map[string]map[string]string{}, mounts: map[string]string{}, vfs: map[string]map[string]any{}, opts: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, &rcClient{http: srv.Client(), base: srv.URL + "/"}
}

type rcIn struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
	Opt        map[string]any  `json:"opt"`
	Fs         string          `json:"fs"`
	MountPoint string          `json:"mountPoint"`
	VfsOpt     map[string]any  `json:"vfsOpt"`
	MountOpt   map[string]any  `json:"mountOpt"`
	Clear      string          `json:"clear"`
	Remote     string          `json:"remote"`
}

func (f *fakeRC) fail(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg, "status": 500})
}

func (f *fakeRC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	method := strings.TrimPrefix(r.URL.Path, "/")
	f.calls = append(f.calls, method)
	var in rcIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "failed to read input JSON: " + err.Error()})
		return
	}
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch method {
	case "config/providers":
		_, _ = w.Write([]byte(fakeProviders))
	case "core/obscure":
		reply(map[string]string{"obscured": "obs(" + in.Clear + ")"})
	case "config/create", "config/update":
		if method == "config/create" {
			if _, ok := f.sections[in.Name]; ok {
				f.fail(w, "remote already exists")
				return
			}
			f.sections[in.Name] = map[string]string{"type": in.Type}
		}
		sec, ok := f.sections[in.Name]
		if !ok {
			f.fail(w, "didn't find section in config file")
			return
		}
		if in.Opt["noObscure"] != true || in.Opt["nonInteractive"] != true {
			f.t.Errorf("%s without noObscure and nonInteractive: %v", method, in.Opt)
		}
		var params map[string]string
		_ = json.Unmarshal(in.Parameters, &params)
		maps.Copy(sec, params)
		if sec["type"] != "drive" {
			reply(map[string]string{"State": ""})
			return
		}
		reply(f.driveQuestions(sec, in))
	case "config/get":
		sec, ok := f.sections[in.Name]
		if !ok {
			f.fail(w, "didn't find section in config file")
			return
		}
		reply(sec)
	case "config/dump":
		reply(f.sections)
	case "config/listremotes":
		reply(map[string][]string{"remotes": slices.Sorted(maps.Keys(f.sections))})
	case "config/delete":
		delete(f.sections, in.Name)
		reply(map[string]any{})
	case "config/unset":
		var keys []string
		_ = json.Unmarshal(in.Parameters, &keys)
		for _, k := range keys {
			delete(f.sections[in.Name], k)
		}
		reply(map[string]any{})
	case "fscache/clear":
		reply(map[string]any{})
	case "mount/mount":
		name, _, _ := strings.Cut(in.Fs, ":")
		if _, ok := f.sections[name]; !ok {
			f.fail(w, "didn't find section in config file")
			return
		}
		f.mounts[in.MountPoint] = in.Fs
		f.vfs[in.MountPoint] = in.VfsOpt
		f.opts[in.MountPoint] = in.MountOpt
		reply(map[string]any{})
	case "mount/unmount":
		if f.unmountRefuse {
			f.fail(w, "device or resource busy")
			return
		}
		delete(f.mounts, in.MountPoint)
		reply(map[string]any{})
	case "mount/listmounts":
		var out []map[string]string
		for p, fs := range f.mounts {
			out = append(out, map[string]string{"Fs": fs, "MountPoint": p})
		}
		reply(map[string]any{"mountPoints": out})
	case "operations/list":
		name, _, _ := strings.Cut(in.Fs, ":")
		if f.sections[name]["host"] == "down" {
			f.fail(w, "couldn't connect SSH: dial tcp: connection refused")
			return
		}
		reply(map[string]any{"list": []map[string]any{{"Name": "a"}, {"Name": "b"}}})
	case "operations/about":
		reply(map[string]int64{"total": 100, "used": 40, "free": 60})
	default:
		f.t.Errorf("unexpected call %s", method)
		f.fail(w, "unknown method")
	}
}

// driveQuestions plays rclone's Google Drive config: confirm the token,
// then the shared drive question.
func (f *fakeRC) driveQuestions(sec map[string]string, in rcIn) map[string]any {
	ask := func(state, name string, def string, required bool) map[string]any {
		return map[string]any{"State": state, "Option": map[string]any{"Name": name, "DefaultStr": def, "ValueStr": def, "Required": required, "Help": name + " help"}}
	}
	if in.Opt["continue"] != true {
		if sec["token"] != "" {
			return ask("*oauth-confirm", "config_refresh_token", "true", false)
		}
		return ask("*oauth-islocal", "config_is_local", "true", false)
	}
	result, _ := in.Opt["result"].(string)
	switch in.Opt["state"] {
	case "*oauth-confirm", "*oauth-islocal":
		if result != "false" {
			f.t.Errorf("%v answered %q, which would open a browser", in.Opt["state"], result)
		}
		if sec["token"] == "" {
			return ask("*oauth-token", "config_token", "", true)
		}
	case "*oauth-token":
		sec["token"] = result
	case "teamdrive":
		return map[string]any{"State": ""}
	}
	return ask("teamdrive", "config_change_team_drive", "false", false)
}

func (f *fakeRC) section(name string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.sections[name])
}

func (f *fakeRC) mounted(p string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.mounts[p]
	return ok
}

type fixture struct {
	t    *testing.T
	h    http.Handler
	s    *Service
	rc   *fakeRC
	root string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(base, "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Root: filepath.Join(base, "remotes")}
	mounting := func(context.Context) (system.Mounting, error) {
		return system.Mounting{Automount: true, Cache: system.CacheWrites, CacheSize: 10}, nil
	}
	s := newService(cfg, store{db}, mounting)
	fake, client := newFakeRC(t)
	s.rc = client

	people := map[string]*users.User{"jdp": {ID: 1}, "anna": {ID: 2}}
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
	router := v1.NewRouter(v1.Deps{DB: db}, auth)
	router.Contract("remotes", contract)
	bind(router, s)
	h, err := router.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, h: h, s: s, rc: fake, root: cfg.Root}
}

func (f *fixture) call(who, method, target string, body any, out any) int {
	f.t.Helper()
	var rd *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("X-User", who)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if out != nil && rec.Code < 300 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			f.t.Fatalf("%s %s: %v in %s", method, target, err, rec.Body)
		}
	}
	if rec.Code >= 400 {
		f.t.Logf("%s %s: %d %s", method, target, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	return rec.Code
}
