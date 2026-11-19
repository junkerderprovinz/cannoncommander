package storeroom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// engine is a docker engine in memory. It answers the calls the storeroom
// makes and keeps a log of every call that changes something.
type engine struct {
	mu         sync.Mutex
	containers []container
	networks   []network
	volumes    []volume
	df         usage
	// store is the engine's driver-type, io.containerd.snapshotter.v1 for
	// containerd's image store.
	store     string
	manifests map[string][]manifest
	changes   []string
}

func (e *engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && p == "/containers/json":
		reply(e.containers)
	case r.Method == http.MethodGet && p == "/networks":
		reply(e.networks)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/networks/"):
		for _, n := range e.networks {
			if n.ID == strings.TrimPrefix(p, "/networks/") {
				reply(n)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	case r.Method == http.MethodGet && p == "/volumes":
		reply(map[string]any{"Volumes": e.volumes})
	case r.Method == http.MethodGet && p == "/system/df":
		reply(e.df)
	case r.Method == http.MethodGet && p == "/info":
		reply(map[string]any{"DriverStatus": [][2]string{{"driver-type", e.store}}})
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/images/") && strings.HasSuffix(p, "/json"):
		reply(map[string]any{"Manifests": e.manifests[strings.TrimSuffix(strings.TrimPrefix(p, "/images/"), "/json")]})
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/rename"):
		id, name := strings.TrimSuffix(strings.TrimPrefix(p, "/containers/"), "/rename"), r.URL.Query().Get("name")
		for i, c := range e.containers {
			if c.name() == name {
				w.WriteHeader(http.StatusConflict)
				return
			}
			if c.ID == id {
				e.containers[i].Names = []string{"/" + name}
			}
		}
		e.changes = append(e.changes, "rename "+id+" "+name)
	case r.Method == http.MethodDelete && strings.HasPrefix(p, "/containers/"):
		e.changes = append(e.changes, "rm container "+strings.TrimPrefix(p, "/containers/"))
	case r.Method == http.MethodDelete && strings.HasPrefix(p, "/networks/"):
		id := strings.TrimPrefix(p, "/networks/")
		for i, n := range e.networks {
			if n.ID == id {
				e.networks = append(e.networks[:i], e.networks[i+1:]...)
				break
			}
		}
		e.changes = append(e.changes, "rm network "+id)
	case r.Method == http.MethodPost && p == "/networks/create":
		var n network
		_ = json.NewDecoder(r.Body).Decode(&n)
		n.ID = "new-" + n.Name
		e.networks = append(e.networks, n)
		e.changes = append(e.changes, "create network "+n.Name+" "+n.Driver+" "+string(n.IPAM))
	case r.Method == http.MethodDelete && strings.HasPrefix(p, "/volumes/"):
		e.changes = append(e.changes, "rm volume "+strings.TrimPrefix(p, "/volumes/"))
	case r.Method == http.MethodDelete && strings.HasPrefix(p, "/images/"):
		e.changes = append(e.changes, "rm image "+strings.TrimPrefix(p, "/images/"))
	case r.Method == http.MethodPost && p == "/build/prune":
		e.changes = append(e.changes, "prune build cache")
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (e *engine) log() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.changes...)
}

// box is a server on disk: the flash with its templates and plugins, the
// appdata share and the storeroom, with an engine in memory beside it.
type box struct {
	t      *testing.T
	root   string
	cfg    Config
	engine *engine
	svc    *Service
}

var now = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

const day = 24 * time.Hour

func newBox(t *testing.T) *box {
	t.Helper()
	root := t.TempDir()
	b := &box{t: t, root: root, engine: &engine{}}
	b.cfg = Config{
		Templates: filepath.Join(root, "boot/config/plugins/dockerMan/templates-user"),
		Plugins:   filepath.Join(root, "boot/config/plugins"),
		Stacks:    filepath.Join(root, "boot/config/plugins/compose.manager/projects"),
		Appdata:   filepath.Join(root, "mnt/user/appdata"),
		Room:      filepath.Join(root, "config/storeroom"),
		Write:     true,
	}
	for _, d := range []string{b.cfg.Templates, b.cfg.Appdata} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(b.engine)
	t.Cleanup(srv.Close)
	b.svc = New(b.cfg)
	b.svc.docker = &docker{c: srv.Client(), base: srv.URL}
	b.svc.now = func() time.Time { return now }
	return b
}

func (b *box) write(path, body string, age time.Duration) string {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.age(path, age)
	return path
}

func (b *box) age(path string, age time.Duration) {
	b.t.Helper()
	at := now.Add(-age)
	if err := os.Chtimes(path, at, at); err != nil {
		b.t.Fatal(err)
	}
}

func (b *box) template(name, body string) string {
	return b.write(filepath.Join(b.cfg.Templates, "my-"+name+".xml"), "<?xml version=\"1.0\"?>\n<Container version=\"2\">\n  <Name>"+name+"</Name>\n"+body+"</Container>\n", 90*day)
}

// appdata makes a folder in appdata with one file, both last changed age ago.
func (b *box) appdata(name string, age time.Duration) string {
	dir := filepath.Join(b.cfg.Appdata, name)
	b.write(filepath.Join(dir, "settings.json"), "{}", age)
	b.age(dir, age)
	return dir
}

// stack makes a compose stack as the compose plugin keeps it on the flash: a
// folder with its compose file and beside it pairs of file name and content.
// Without a compose file the stack lives elsewhere, as an indirect one does.
func (b *box) stack(folder, compose string, age time.Duration, files ...string) string {
	dir := filepath.Join(b.cfg.Stacks, folder)
	if compose != "" {
		b.write(filepath.Join(dir, "docker-compose.yml"), compose, age)
	}
	for i := 0; i+1 < len(files); i += 2 {
		b.write(filepath.Join(dir, files[i]), files[i+1], age)
	}
	b.age(dir, age)
	return dir
}

func ctr(id, name, state string, age time.Duration, labels map[string]string, mounts ...string) container {
	c := container{ID: id, Names: []string{"/" + name}, State: state, Created: now.Add(-age).Unix(), Labels: labels}
	for _, m := range mounts {
		c.Mounts = append(c.Mounts, mount{Type: "bind", Source: m})
	}
	return c
}

func (b *box) scan() *Scan {
	b.t.Helper()
	s, err := b.svc.Scan(b.t.Context())
	if err != nil {
		b.t.Fatal(err)
	}
	return s
}

// found lists the finds of one kind by name.
func found(s *Scan, kind string) []string {
	out := []string{}
	for _, f := range s.Finds {
		if f.Kind == kind {
			out = append(out, f.Name)
		}
	}
	return out
}

func find(t *testing.T, s *Scan, kind, name string) Ref {
	t.Helper()
	for _, f := range s.Finds {
		if f.Kind == kind && f.Name == name {
			return Ref{Kind: f.Kind, ID: f.ID}
		}
	}
	t.Fatalf("no %s %s in %+v", kind, name, s.Finds)
	return Ref{}
}
