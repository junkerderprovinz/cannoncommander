//go:build desktop

package shellapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/nativeshell"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/elevate"
	_ "modernc.org/sqlite"
)

type hiddenRules struct{ path string }

func (r hiddenRules) Effect(_ *users.User, p string) ccfs.Effect {
	if p == r.path || strings.HasPrefix(p, r.path+"/") {
		return ccfs.Hidden
	}
	return ccfs.Open
}

// computer stands in for the system and remembers what it was asked.
type computer struct {
	mu       sync.Mutex
	native   bool
	menuRows []nativeshell.Entry
	shown    [][]string
	hiddenAt map[string]bool
	actions  []nativeshell.Action
	ran      []string
	elevated []string
	props    [][]string
	// sandbox is the state of the thumbnail sandbox, which the operations
	// asked for with administrator rights change, unless decline is set.
	sandbox     nativeshell.SandboxState
	asked       []elevate.Op
	resandboxed int
	decline     bool
}

func (c *computer) system() system {
	return system{
		nativeMenu: func() bool { return c.native },
		showMenu: func(files []string, o nativeshell.MenuOptions) ([]nativeshell.Entry, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.shown = append(c.shown, files)
			c.hiddenAt = map[string]bool{}
			for _, e := range c.menuRows {
				c.hiddenAt[e.Key] = o.Hidden(e.Key)
			}
			return c.menuRows, nil
		},
		actions: func([]string) ([]nativeshell.Action, error) { return c.actions, nil },
		runAction: func(files []string, key string) error {
			if !slices.ContainsFunc(c.actions, func(a nativeshell.Action) bool { return a.Key == key }) {
				return nativeshell.ErrUnknownAction
			}
			c.ran = append(c.ran, key)
			return nil
		},
		hasProperties: func() bool { return c.native },
		properties: func(files []string) error {
			c.props = append(c.props, files)
			return nil
		},
		canRunAsAdmin: func(f string) bool { return strings.HasSuffix(f, ".exe") || strings.HasSuffix(f, ".cmd") },
		runAsAdmin: func(f string) error {
			c.elevated = append(c.elevated, f)
			return nil
		},
		icon: func(f string, px int) ([]byte, string, error) {
			if strings.HasSuffix(f, ".bin") {
				return nil, "", nativeshell.ErrNoPicture
			}
			return []byte("icon of " + filepath.Base(f)), "image/png", nil
		},
		sandboxing: func() nativeshell.SandboxState { return c.sandbox },
		resandbox:  func() { c.resandboxed++ },
		elevate: func(_ context.Context, ops []elevate.Op) error {
			if c.decline {
				return elevate.ErrDeclined
			}
			c.asked = append(c.asked, ops...)
			c.sandbox.Allowed = ops[0].Kind == elevate.AllowUserNamespaces
			c.sandbox.Sandboxed = c.sandbox.Allowed
			return nil
		},
		typeIcon: func(name string, dir bool, px int) ([]byte, string, error) {
			return []byte("type of " + name), "image/png", nil
		},
	}
}

type env struct {
	t    *testing.T
	root string
	srv  *httptest.Server
	pc   *computer
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "main.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var mine []migrate.Migration
	for _, m := range migrate.Registered(migrate.Main) {
		if strings.HasPrefix(m.Name, "shellapi/") {
			mine = append(mine, m)
		}
	}
	if _, err := migrate.ApplyList(context.Background(), db, mine); err != nil {
		t.Fatal(err)
	}
	return db
}

func newEnv(t *testing.T, native bool) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"work", "other", "private"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"work/notes.txt", "work/setup.exe", "work/data.bin", "other/b.txt", "private/diary.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	fsys, err := ccfs.New(hiddenRules{"/mnt/user/private"}, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	user := func(name string) (*users.User, error) {
		return &users.User{Username: name, BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: all}}}, nil
	}
	deps := v1.Deps{FS: fsys, Users: user, DB: openDB(t)}
	r := v1.NewRouter(deps, func(http.ResponseWriter, *http.Request) (*users.User, error) { return user("jdp") })
	r.Contract("shell", contract)
	e := &env{t: t, root: root, pc: &computer{native: native}}
	bind(r, &handlers{deps: deps, sys: e.pc.system(), seen: entries{deps.DB}})
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *env) do(method, target string, body any) (*http.Response, string, []byte) {
	e.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, e.srv.URL+target, bytes.NewReader(raw))
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	var p v1.Problem
	_ = json.Unmarshal(b.Bytes(), &p)
	return resp, p.Code, b.Bytes()
}

func (e *env) entries() []entryJSON {
	e.t.Helper()
	_, _, body := e.do(http.MethodGet, "/shell/entries", nil)
	var out struct{ Entries []entryJSON }
	if err := json.Unmarshal(body, &out); err != nil {
		e.t.Fatal(err)
	}
	return out.Entries
}

func TestTheOffersNameTheProgramsThatRunAsAdministrator(t *testing.T) {
	e := newEnv(t, true)
	_, _, body := e.do(http.MethodGet, "/shell", nil)
	var got offersJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Menu != "native" || !got.Properties || !slices.Equal(got.RunAs, []string{".exe", ".cmd"}) {
		t.Fatalf("got %+v", got)
	}
	linux := newEnv(t, false)
	_, _, body = linux.do(http.MethodGet, "/shell", nil)
	_ = json.Unmarshal(body, &got)
	if got.Menu != "actions" {
		t.Fatalf("got %+v", got)
	}
}

func TestAHiddenRowStaysOutOfTheNextMenuAndCanComeBack(t *testing.T) {
	e := newEnv(t, true)
	e.pc.menuRows = []nativeshell.Entry{{Key: "verb:7-zip", Label: "7-Zip", Icon: "data:image/png;base64,AA=="}, {Key: "text:Senden an", Label: "Senden an"}}
	menu := map[string]any{"paths": []string{"/mnt/user/work/notes.txt", "/mnt/user/work/setup.exe"}, "x": 10, "y": 20}
	if resp, code, _ := e.do(http.MethodPost, "/shell/menu", menu); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("got %d %s", resp.StatusCode, code)
	}
	want := []string{filepath.Join(e.root, "work", "notes.txt"), filepath.Join(e.root, "work", "setup.exe")}
	if !slices.Equal(e.pc.shown[0], want) {
		t.Fatalf("showed the menu for %v", e.pc.shown[0])
	}
	if got := e.entries(); len(got) != 2 || got[0].Label != "7-Zip" || got[0].Icon == "" || got[0].Hidden {
		t.Fatalf("got %+v", got)
	}

	if resp, _, _ := e.do(http.MethodPut, "/shell/entries/hidden", map[string]any{"keys": []string{"verb:7-zip"}}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("got %d", resp.StatusCode)
	}
	e.do(http.MethodPost, "/shell/menu", menu)
	if !e.pc.hiddenAt["verb:7-zip"] || e.pc.hiddenAt["text:Senden an"] {
		t.Fatalf("hidden %v", e.pc.hiddenAt)
	}
	if got := e.entries(); !got[0].Hidden {
		t.Fatal("the hidden row is listed as shown")
	}

	e.do(http.MethodPut, "/shell/entries/hidden", map[string]any{"keys": []string{}})
	e.do(http.MethodPost, "/shell/menu", menu)
	if e.pc.hiddenAt["verb:7-zip"] {
		t.Fatal("the row stayed hidden after showing them all")
	}
}

func TestAMenuIsForItemsOfOneFolderTheRulesShow(t *testing.T) {
	e := newEnv(t, true)
	for paths, want := range map[string]string{
		"/mnt/user/work/notes.txt,/mnt/user/other/b.txt": "notOneFolder",
		"/mnt/user/private/diary.txt":                    "",
	} {
		resp, code, _ := e.do(http.MethodPost, "/shell/menu", map[string]any{"paths": strings.Split(paths, ","), "x": 0, "y": 0})
		if resp.StatusCode < 400 || (want != "" && code != want) {
			t.Errorf("%s: got %d %s", paths, resp.StatusCode, code)
		}
	}
	if len(e.pc.shown) != 0 {
		t.Fatalf("showed %v", e.pc.shown)
	}
}

func TestHiddenActionsDropOutAndUnknownOnesDoNotRun(t *testing.T) {
	e := newEnv(t, false)
	e.pc.actions = []nativeshell.Action{
		{Entry: nativeshell.Entry{Key: "nemo:shrink", Label: "Shrink"}, Group: "Images"},
		{Entry: nativeshell.Entry{Key: "thunar:mail", Label: "Mail"}},
	}
	// A row can be hidden once a list has seen it.
	e.do(http.MethodGet, "/shell/actions?path=/mnt/user/work/notes.txt", nil)
	e.do(http.MethodPut, "/shell/entries/hidden", map[string]any{"keys": []string{"thunar:mail"}})
	_, _, body := e.do(http.MethodGet, "/shell/actions?path=/mnt/user/work/notes.txt", nil)
	var got struct{ Actions []actionJSON }
	_ = json.Unmarshal(body, &got)
	if len(got.Actions) != 1 || got.Actions[0].Key != "nemo:shrink" || got.Actions[0].Group != "Images" {
		t.Fatalf("got %+v", got.Actions)
	}
	if labels := e.entries(); labels[0].Label != "Images › Shrink" {
		t.Fatalf("got %+v", labels)
	}
	if resp, code, _ := e.do(http.MethodPost, "/shell/actions", map[string]any{"paths": []string{"/mnt/user/work/notes.txt"}, "key": "rm"}); resp.StatusCode != http.StatusBadRequest || code != "unknownAction" {
		t.Fatalf("got %d %s", resp.StatusCode, code)
	}
	if resp, _, _ := e.do(http.MethodPost, "/shell/actions", map[string]any{"paths": []string{"/mnt/user/work/notes.txt"}, "key": "nemo:shrink"}); resp.StatusCode != http.StatusNoContent || !slices.Equal(e.pc.ran, []string{"nemo:shrink"}) {
		t.Fatalf("got %d, ran %v", resp.StatusCode, e.pc.ran)
	}
}

func TestOnlyAProgramStartsAsAdministrator(t *testing.T) {
	e := newEnv(t, true)
	for _, p := range []string{"/mnt/user/work/notes.txt", "/mnt/user/work"} {
		if resp, code, _ := e.do(http.MethodPost, "/shell/runas", map[string]string{"path": p}); resp.StatusCode != http.StatusBadRequest || code != "notAProgram" {
			t.Errorf("%s: got %d %s", p, resp.StatusCode, code)
		}
	}
	if resp, _, _ := e.do(http.MethodPost, "/shell/runas", map[string]string{"path": "/mnt/user/work/setup.exe"}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("got %d", resp.StatusCode)
	}
	if !slices.Equal(e.pc.elevated, []string{filepath.Join(e.root, "work", "setup.exe")}) {
		t.Fatalf("got %v", e.pc.elevated)
	}
}

func TestPropertiesNeedADialog(t *testing.T) {
	linux := newEnv(t, false)
	if resp, code, _ := linux.do(http.MethodPost, "/shell/properties", map[string]any{"paths": []string{"/mnt/user/work/notes.txt"}}); resp.StatusCode != http.StatusNotFound || code != "noPropertiesDialog" {
		t.Fatalf("got %d %s", resp.StatusCode, code)
	}
	e := newEnv(t, true)
	if resp, _, _ := e.do(http.MethodPost, "/shell/properties", map[string]any{"paths": []string{"/mnt/user/work"}}); resp.StatusCode != http.StatusNoContent || len(e.pc.props) != 1 {
		t.Fatalf("got %d %v", resp.StatusCode, e.pc.props)
	}
}

func TestAnIconComesByTypeOrByItem(t *testing.T) {
	e := newEnv(t, true)
	resp, _, body := e.do(http.MethodGet, "/shell/icon?size=32&name=../../secret/report.pdf", nil)
	if resp.StatusCode != http.StatusOK || string(body) != "type of report.pdf" || !strings.Contains(resp.Header.Get("Cache-Control"), "max-age=86400") {
		t.Fatalf("got %d %q %s", resp.StatusCode, body, resp.Header.Get("Cache-Control"))
	}
	resp, _, body = e.do(http.MethodGet, "/shell/icon?size=48&path=/mnt/user/work/setup.exe&v=7", nil)
	if string(body) != "icon of setup.exe" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("got %q %v", body, resp.Header)
	}
	if resp, _, _ := e.do(http.MethodGet, "/shell/icon?size=32&path=/mnt/user/work/data.bin", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("an item without an icon got %d", resp.StatusCode)
	}
	if resp, _, _ := e.do(http.MethodGet, "/shell/icon?size=20&name=a.txt", nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("an odd size got %d", resp.StatusCode)
	}
}

func TestTheCommonIconsAreLookedUpForEverySizeOfARow(t *testing.T) {
	asked := map[string]int{}
	sys := system{typeIcon: func(name string, dir bool, px int) ([]byte, string, error) {
		asked[name+"@"+strconv.Itoa(px)]++
		return nil, "", nativeshell.ErrNoPicture
	}}
	(&handlers{sys: sys}).warmIcons()
	for _, want := range []string{"file.png@16", "file.pdf@48", "file.mp4@32", "folder@16"} {
		if asked[want] != 1 {
			t.Errorf("%s asked %d times", want, asked[want])
		}
	}
}

func TestTheSettingsKeepALabelWithoutTheFileNames(t *testing.T) {
	e := newEnv(t, false)
	e.pc.actions = []nativeshell.Action{{Entry: nativeshell.Entry{Key: "nemo:scan", Label: "Scan notes.txt", Remember: "Scan %N"}}}
	_, _, body := e.do(http.MethodGet, "/shell/actions?path=/mnt/user/work/notes.txt", nil)
	var got struct{ Actions []actionJSON }
	_ = json.Unmarshal(body, &got)
	if len(got.Actions) != 1 || got.Actions[0].Label != "Scan notes.txt" {
		t.Fatalf("the menu shows %+v", got.Actions)
	}
	if kept := e.entries(); len(kept) != 1 || kept[0].Label != "Scan %N" {
		t.Fatalf("kept %+v", kept)
	}
}

func TestThumbnailsAreLetIntoTheSandboxAndOutAgainWithAdministratorRights(t *testing.T) {
	e := newEnv(t, false)
	e.pc.sandbox = nativeshell.SandboxState{Restricted: true}
	if runtime.GOOS == "linux" {
		_, _, body := e.do(http.MethodGet, "/shell", nil)
		var got offersJSON
		_ = json.Unmarshal(body, &got)
		if got.Thumbnails == nil || got.Thumbnails.Way != "cache" || !got.Thumbnails.Restricted {
			t.Fatalf("offers %s", body)
		}
	}
	for _, c := range []struct {
		allow bool
		way   string
	}{{true, "sandbox"}, {false, "cache"}} {
		resp, code, body := e.do(http.MethodPost, "/shell/thumbnails", map[string]bool{"allow": c.allow})
		var got thumbsJSON
		_ = json.Unmarshal(body, &got)
		if resp.StatusCode != http.StatusOK || got.Way != c.way || got.Allowed != c.allow {
			t.Fatalf("allow %v: %d %s %s", c.allow, resp.StatusCode, code, body)
		}
	}
	var kinds []string
	for _, o := range e.pc.asked {
		kinds = append(kinds, o.Kind)
	}
	if want := []string{elevate.AllowUserNamespaces, elevate.DisallowUserNamespaces}; !slices.Equal(kinds, want) || e.pc.resandboxed != 2 {
		t.Fatalf("asked %+v, probed again %d times", e.pc.asked, e.pc.resandboxed)
	}
	e.pc.decline = true
	if resp, code, _ := e.do(http.MethodPost, "/shell/thumbnails", map[string]bool{"allow": true}); resp.StatusCode != http.StatusForbidden || code != "elevationDeclined" {
		t.Fatalf("a refused prompt: %d %s", resp.StatusCode, code)
	}
}
