//go:build desktop

package places

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// computer is a desktop app with drives that the test plugs in and pulls.
type computer struct {
	mu      sync.Mutex
	vols    []desktop.Volume
	changed func()
	ejected []string
	mounted []string
	fail    error
	mountAt string
}

func (*computer) AutoUpdate() bool         { return false }
func (*computer) SetAutoUpdate(bool) error { return nil }
func (*computer) UpdateReady() string      { return "" }
func (*computer) Restart()                 {}

func (c *computer) Volumes() []desktop.Volume {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.vols)
}

func (c *computer) OnVolumes(changed func()) { c.changed = changed }

func (c *computer) Eject(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail != nil {
		return c.fail
	}
	c.ejected = append(c.ejected, id)
	return nil
}

func (c *computer) Mount(id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mounted = append(c.mounted, id)
	return c.mountAt, nil
}

// plug sets the drives and tells whoever follows them.
func (c *computer) plug(vols ...desktop.Volume) {
	c.mu.Lock()
	c.vols = vols
	c.mu.Unlock()
	c.changed()
}

func onDesktop(t *testing.T, home string) *computer {
	t.Helper()
	c := &computer{}
	desktop.Use(c, desktop.Options{Home: home})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
	return c
}

type env struct {
	t    *testing.T
	home string
	srv  *httptest.Server
}

func newEnv(t *testing.T, known func(home string) []folder) *env {
	t.Helper()
	home := t.TempDir()
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Home", Mount: apiPath(home), Root: home})
	if err != nil {
		t.Fatal(err)
	}
	admin := &users.User{Username: "jdp"}
	admin.Permissions.Admin = true
	r := v1.NewRouter(v1.Deps{FS: fs}, func(http.ResponseWriter, *http.Request) (*users.User, error) { return admin, nil })
	r.Contract("places", contract)
	bind(r, handlers{fs: fs, known: known})
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, home: home, srv: httptest.NewServer(handler)}
	t.Cleanup(e.srv.Close)
	return e
}

func (e *env) do(method, target string, body any) (int, string, []byte) {
	e.t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(method, e.srv.URL+target, bytes.NewReader(raw))
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	var p v1.Problem
	_ = json.Unmarshal(b.Bytes(), &p)
	return resp.StatusCode, p.Code, b.Bytes()
}

func TestPlacesAreTheFoldersThatExistAndTheDrives(t *testing.T) {
	var home string
	e := newEnv(t, func(h string) []folder {
		return []folder{{"home", h}, {"documents", filepath.Join(h, "Dokumente")}, {"music", filepath.Join(h, "Musik")}}
	})
	home = e.home
	if err := os.Mkdir(filepath.Join(home, "Dokumente"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := onDesktop(t, home)
	stick := t.TempDir()
	c.vols = []desktop.Volume{
		{ID: "E:", Root: stick, Label: "Urlaub", Kind: "removable", Eject: true},
		{ID: "sdb2", Kind: "fixed", Size: 16e9},
	}

	status, _, body := e.do(http.MethodGet, "/places", nil)
	var got listJSON
	if err := json.Unmarshal(body, &got); status != http.StatusOK || err != nil {
		t.Fatalf("%d %v", status, err)
	}
	want := []folderJSON{{Kind: "home", Path: apiPath(home)}, {Kind: "documents", Path: apiPath(filepath.Join(home, "Dokumente"))}}
	if !slices.Equal(got.Folders, want) {
		t.Errorf("folders %+v", got.Folders)
	}
	if len(got.Volumes) != 2 || got.Volumes[0].Path != apiPath(stick) || got.Volumes[0].Label != "Urlaub" || got.Volumes[1].Path != "" {
		t.Errorf("volumes %+v", got.Volumes)
	}
}

func TestEjectHandsTheDriveToTheComputer(t *testing.T) {
	e := newEnv(t, func(string) []folder { return nil })
	c := onDesktop(t, e.home)
	c.vols = []desktop.Volume{{ID: "C:", Root: `C:\`, Kind: "fixed"}, {ID: "E:", Root: `E:\`, Kind: "removable", Eject: true}}

	if status, _, _ := e.do(http.MethodPost, "/places/eject", map[string]string{"id": "E:"}); status != http.StatusNoContent || !slices.Equal(c.ejected, []string{"E:"}) {
		t.Fatalf("%d, ejected %v", status, c.ejected)
	}
	if status, code, _ := e.do(http.MethodPost, "/places/eject", map[string]string{"id": "C:"}); status != http.StatusConflict || code != "ejectFailed" {
		t.Errorf("a fixed drive: %d %s", status, code)
	}
	if status, code, _ := e.do(http.MethodPost, "/places/eject", map[string]string{"id": "Z:"}); status != http.StatusNotFound || code != "noVolume" {
		t.Errorf("no such drive: %d %s", status, code)
	}
	c.fail = errors.New("in use")
	if status, code, _ := e.do(http.MethodPost, "/places/eject", map[string]string{"id": "E:"}); status != http.StatusConflict || code != "ejectFailed" {
		t.Errorf("a drive in use: %d %s", status, code)
	}
}

func TestMountAnswersWhereTheVolumeIs(t *testing.T) {
	e := newEnv(t, func(string) []folder { return nil })
	c := onDesktop(t, e.home)
	stick, disk := t.TempDir(), t.TempDir()
	c.mountAt = stick
	c.vols = []desktop.Volume{{ID: "sdb1", Kind: "removable"}, {ID: "sda3", Root: disk, Kind: "fixed"}}

	status, _, body := e.do(http.MethodPost, "/places/mount", map[string]string{"id": "sdb1"})
	var got mountedJSON
	_ = json.Unmarshal(body, &got)
	if status != http.StatusOK || got.Path != apiPath(stick) || !slices.Equal(c.mounted, []string{"sdb1"}) {
		t.Fatalf("%d %+v, mounted %v", status, got, c.mounted)
	}
	_, _, body = e.do(http.MethodPost, "/places/mount", map[string]string{"id": "sda3"})
	_ = json.Unmarshal(body, &got)
	if got.Path != apiPath(disk) || len(c.mounted) != 1 {
		t.Fatalf("a mounted volume: %+v, mounted %v", got, c.mounted)
	}
}

func TestADriveThatComesAndGoesReachesTheResolverAndTheWindow(t *testing.T) {
	home, stick := t.TempDir(), t.TempDir()
	c := onDesktop(t, home)
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Home", Mount: apiPath(home), Root: home})
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(hub.Options{})
	client, _, _ := h.Connect("tab", "jdp", 0)
	h.Subscribe("tab", "jdp", hub.FolderTopic(apiPath(stick)))
	desktop.Listen(func(typ string, data any) { _, _ = h.PublishJSON(typ, "", data) })
	t.Cleanup(func() { desktop.Listen(nil) })
	fs = Follow(fs, h)

	c.plug(desktop.Volume{ID: "E:", Root: stick, Kind: "removable", Eject: true})
	if _, err := fs.Resolve(apiPath(stick) + "/fotos"); err != nil {
		t.Fatalf("the stick is no source: %v", err)
	}
	if ev := next(t, client.Events, "desktop.volumes"); !bytes.Contains(ev.Data, []byte(`"added":[{"id":"E:"`)) {
		t.Errorf("event %s", ev.Data)
	}

	c.plug()
	if _, err := fs.Resolve(apiPath(stick) + "/fotos"); !errors.Is(err, ccfs.ErrNotFound) {
		t.Fatalf("the pulled stick still resolves: %v", err)
	}
	if ev := next(t, client.Events, "desktop.volumes"); !bytes.Contains(ev.Data, []byte(`"removed":[{"id":"E:"`)) {
		t.Errorf("event %s", ev.Data)
	}
	var ch watch.Change
	if err := json.Unmarshal(next(t, client.Events, watch.EventChanged).Data, &ch); err != nil || !ch.Gone || ch.Path != apiPath(stick) {
		t.Errorf("change %+v, %v", ch, err)
	}
}

func TestAVolumeComesWhenItAppearsAndGoesWhenItIsUnmounted(t *testing.T) {
	before := []desktop.Volume{{ID: "a", Root: "/run/media/jdp/A"}, {ID: "b"}, {ID: "c", Root: "/run/media/jdp/C"}}
	now := []desktop.Volume{{ID: "a"}, {ID: "b", Root: "/run/media/jdp/B"}, {ID: "d"}}
	added, removed := compare(before, now)
	ids := func(vs []desktop.Volume) (out []string) {
		for _, v := range vs {
			out = append(out, v.ID)
		}
		return out
	}
	if !slices.Equal(ids(added), []string{"d"}) || !slices.Equal(ids(removed), []string{"a", "c"}) {
		t.Fatalf("added %v, removed %v", ids(added), ids(removed))
	}
}

func TestAStickTheDesktopMountsAfterItCameIsGreetedOnce(t *testing.T) {
	home, stick := t.TempDir(), t.TempDir()
	c := onDesktop(t, home)
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Home", Mount: apiPath(home), Root: home})
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(hub.Options{})
	client, _, _ := h.Connect("tab", "jdp", 0)
	desktop.Listen(func(typ string, data any) { _, _ = h.PublishJSON(typ, "", data) })
	t.Cleanup(func() { desktop.Listen(nil) })
	Follow(fs, h)

	c.plug(desktop.Volume{ID: "sdb1", Kind: "removable", Eject: true})
	if ev := next(t, client.Events, "desktop.volumes"); !bytes.Contains(ev.Data, []byte(`"added":[{"id":"sdb1"`)) {
		t.Fatalf("arrival %s", ev.Data)
	}
	c.plug(desktop.Volume{ID: "sdb1", Root: stick, Kind: "removable", Eject: true})
	if ev := next(t, client.Events, "desktop.volumes"); !bytes.Contains(ev.Data, []byte(`"added":[]`)) {
		t.Fatalf("the mount greeted the stick again: %s", ev.Data)
	}
}

func next(t *testing.T, events <-chan hub.Event, typ string) hub.Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == typ {
				return ev
			}
		case <-timeout:
			t.Fatalf("no %s arrived", typ)
		}
	}
}
