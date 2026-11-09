package remotes

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

var errUnavailable = errors.New("rclone is not installed on this server")

// tryPrefix names the sections a connection test creates for a moment.
const tryPrefix = "cc-try-"

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Waits for rcd: a call from the settings page may come right after the
// start, a list should not hang on it.
const (
	readyWait = 20 * time.Second
	glance    = 3 * time.Second
)

// Service manages the remotes. It is safe for concurrent use.
type Service struct {
	cfg      Config
	db       store
	rc       *rcClient
	d        *daemon
	mounting func(context.Context) (system.Mounting, error)
	// readKey asks an SSH server for its key. Tests answer it themselves.
	readKey func(ctx context.Context, host, port string) (HostKey, error)

	start   sync.Once
	stop    context.CancelFunc
	stopped chan struct{}
	// mu lets one change run at a time, so rclone.conf, the table and the
	// mounts agree.
	mu sync.Mutex

	errMu sync.Mutex
	errs  map[string]string

	provMu sync.Mutex
	provs  []provider

	labels atomic.Pointer[map[string]string]
}

// Remote is one configured remote. Params and Secrets are only filled in
// for admins, Secrets names the parameters that are set but never sent.
type Remote struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Provider is the product behind the remote where its parameters name
	// one, for its logo.
	Provider     string            `json:"provider,omitempty"`
	Path         string            `json:"path"`
	Mount        string            `json:"mount"`
	Label        string            `json:"label"`
	Mounted      bool              `json:"mounted"`
	MountOnStart bool              `json:"mountOnStart"`
	Error        string            `json:"error,omitempty"`
	Params       map[string]string `json:"params,omitempty"`
	Secrets      []string          `json:"secrets,omitempty"`
}

// Input is what creating, changing and testing a remote take. Params are
// rclone's parameters of the type. An empty value removes a parameter,
// except for a secret, which then stays as it is.
type Input struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Path   *string           `json:"path"`
	Mount  string            `json:"mount"`
	Params map[string]string `json:"params"`
}

// Status says whether rclone is there and running.
type Status struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

// List is what GET /remotes answers.
type List struct {
	Remotes []Remote `json:"remotes"`
	Rclone  Status   `json:"rclone"`
}

func newService(cfg Config, db store, mounting func(context.Context) (system.Mounting, error)) *Service {
	s := &Service{cfg: cfg, db: db, mounting: mounting, readKey: readHostKey, errs: map[string]string{}}
	if cfg.Binary != "" {
		s.rc = socketClient(cfg.Socket)
		s.d = newDaemon(cfg, s.rc)
		s.d.onUp = s.up
	}
	return s
}

// ready starts rcd on first use and waits for it.
func (s *Service) ready(ctx context.Context, within time.Duration) error {
	if s.rc == nil {
		return errUnavailable
	}
	if s.d == nil {
		return nil
	}
	s.start.Do(func() {
		run, cancel := context.WithCancel(context.Background())
		s.stop, s.stopped = cancel, make(chan struct{})
		go func() {
			defer close(s.stopped)
			s.d.run(run)
		}()
	})
	if s.stop == nil {
		return errNotRunning
	}
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	return s.d.wait(ctx)
}

// Stop ends rcd, which unmounts the remotes first, and keeps it from
// starting again.
func (s *Service) Stop() {
	s.start.Do(func() {})
	if s.stop != nil {
		s.stop()
		<-s.stopped
	}
}

func (s *Service) status() Status {
	if s.rc == nil {
		return Status{}
	}
	if s.d == nil {
		return Status{Installed: true, Running: true}
	}
	up, version, failure := s.d.status()
	return Status{Installed: true, Running: up, Version: version, Error: failure}
}

// boot runs once at server start. With remotes in the table it starts rcd
// right away, which mounts them. Without any it waits for the first call.
func (s *Service) boot(ctx context.Context) {
	rows, err := s.db.all(ctx)
	if err != nil {
		logger.Errorf("remotes: %v", err)
		return
	}
	s.refreshLabels(ctx)
	if len(rows) > 0 && s.rc != nil {
		if err := s.ready(ctx, readyWait); err != nil {
			logger.Errorf("remotes: %v", err)
		}
	}
}

// up runs each time rcd answers. After the first start the automount
// setting decides, after a crash the remotes come back as they were.
func (s *Service) up(ctx context.Context, first bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.tidy(ctx); err != nil {
		logger.Errorf("remotes: %v", err)
	}
	defer s.refreshLabels(ctx)
	m, err := s.mounting(ctx)
	if err != nil {
		logger.Errorf("remotes: %v", err)
		return
	}
	if first && !m.Automount {
		return
	}
	rows, err := s.db.all(ctx)
	if err != nil {
		logger.Errorf("remotes: %v", err)
		return
	}
	for _, r := range rows {
		if !r.MountOnStart {
			continue
		}
		if err := s.mountRow(ctx, r, rows); err != nil {
			logger.Errorf("remotes: mount %s on %s: %v", r.Name, r.Mount, err)
		}
	}
}

// tidy drops the sections of tests that did not finish and takes in
// remotes someone added to rclone.conf by hand.
func (s *Service) tidy(ctx context.Context) error {
	var out struct {
		Remotes []string `json:"remotes"`
	}
	if err := s.rc.call(ctx, "config/listremotes", nil, &out); err != nil {
		return err
	}
	rows, err := s.db.all(ctx)
	if err != nil {
		return err
	}
	for _, name := range out.Remotes {
		if strings.HasPrefix(name, tryPrefix) {
			s.forget(name)
			continue
		}
		if slices.ContainsFunc(rows, func(r row) bool { return r.Name == name }) || !validName.MatchString(name) {
			continue
		}
		section, err := s.section(ctx, name)
		if err != nil {
			return err
		}
		r := row{Name: name, Type: section["type"], Mount: filepath.Join(s.cfg.Root, name)}
		if s.checkMount(r.Mount, "", rows) != nil {
			continue
		}
		if err := s.db.insert(ctx, r); err != nil {
			return err
		}
		rows = append(rows, r)
		logger.Infof("remotes: took in %s from rclone.conf", name)
	}
	return nil
}

func (s *Service) noteError(name string, err error) {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if err == nil {
		delete(s.errs, name)
		return
	}
	s.errs[name] = err.Error()
}

func (s *Service) lastError(name string) string {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.errs[name]
}

// providers returns rclone's backends. The answer is large and only changes
// with rclone, so it is kept.
func (s *Service) providers(ctx context.Context) ([]provider, error) {
	s.provMu.Lock()
	defer s.provMu.Unlock()
	if s.provs != nil {
		return s.provs, nil
	}
	var out struct {
		Providers []provider `json:"providers"`
	}
	if err := s.rc.call(ctx, "config/providers", nil, &out); err != nil {
		return nil, err
	}
	s.provs = out.Providers
	return s.provs, nil
}

func (s *Service) provider(ctx context.Context, typ string) (*provider, error) {
	provs, err := s.providers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range provs {
		if provs[i].Name == typ {
			return &provs[i], nil
		}
	}
	if typ == "" {
		return nil, v1.Errorf(http.StatusBadRequest, "pick a type for the remote")
	}
	return nil, v1.Errorf(http.StatusBadRequest, "rclone does not know the type "+typ)
}

// Types returns the kinds of remotes rclone offers.
func (s *Service) Types(ctx context.Context, advanced bool) ([]Type, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return nil, err
	}
	provs, err := s.providers(ctx)
	if err != nil {
		return nil, err
	}
	return types(provs, advanced), nil
}

func (s *Service) section(ctx context.Context, name string) (map[string]string, error) {
	out := map[string]string{}
	err := s.rc.call(ctx, "config/get", map[string]string{"name": name}, &out)
	return out, err
}

func (s *Service) dump(ctx context.Context) map[string]map[string]string {
	out := map[string]map[string]string{}
	if err := s.rc.call(ctx, "config/dump", nil, &out); err != nil {
		return nil
	}
	return out
}

func (s *Service) mountedPoints(ctx context.Context) map[string]bool {
	var out struct {
		MountPoints []struct {
			MountPoint string `json:"MountPoint"`
		} `json:"mountPoints"`
	}
	if s.rc == nil || s.rc.call(ctx, "mount/listmounts", nil, &out) != nil {
		return map[string]bool{}
	}
	m := map[string]bool{}
	for _, p := range out.MountPoints {
		m[filepath.Clean(p.MountPoint)] = true
	}
	return m
}

func label(r row, params map[string]string) string {
	if r.Type == "smb" && params["host"] != "" {
		share, _, _ := strings.Cut(strings.Trim(r.Path, "/"), "/")
		return "SMB: //" + params["host"] + "/" + share
	}
	return "rclone: " + r.Name
}

// refreshLabels rebuilds the marks the listing and the tree put on mounted
// folders. Before rcd answers they are plain.
func (s *Service) refreshLabels(ctx context.Context) {
	rows, err := s.db.all(ctx)
	if err != nil {
		return
	}
	var dump map[string]map[string]string
	if s.rc != nil && (s.d == nil || s.status().Running) {
		dump = s.dump(ctx)
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[filepath.Clean(r.Mount)] = label(r, dump[r.Name])
	}
	s.labels.Store(&m)
}

// labelAt names the remote mounted at a folder.
func (s *Service) labelAt(p string) string {
	m := s.labels.Load()
	if m == nil {
		return ""
	}
	return (*m)[filepath.Clean(p)]
}

// view shows a remote. Its parameters name the product to everybody, and
// only admins get the parameters themselves.
func (s *Service) view(r row, mounted map[string]bool, params map[string]string, p *provider, admin bool) Remote {
	out := Remote{
		Name: r.Name, Type: r.Type, Provider: providerFor(r.Type, params), Path: r.Path, Mount: r.Mount, Label: label(r, params),
		Mounted: mounted[filepath.Clean(r.Mount)], MountOnStart: r.MountOnStart, Error: s.lastError(r.Name),
	}
	if params == nil || !admin {
		return out
	}
	out.Params = map[string]string{}
	out.Secrets = []string{}
	for k, v := range params {
		switch {
		case k == "type":
		case (p != nil && p.secret(k)) || (p == nil && secretName(k)):
			out.Secrets = append(out.Secrets, k)
		default:
			out.Params[k] = v
		}
	}
	slices.Sort(out.Secrets)
	return out
}

// List returns every remote. Admins also get the parameters.
func (s *Service) List(ctx context.Context, admin bool) (List, error) {
	if admin {
		_ = s.ready(ctx, glance)
	}
	rows, err := s.db.all(ctx)
	if err != nil {
		return List{}, err
	}
	st := s.status()
	mounted := map[string]bool{}
	var dump map[string]map[string]string
	if st.Running {
		mounted = s.mountedPoints(ctx)
		dump = s.dump(ctx)
	}
	out := List{Remotes: []Remote{}, Rclone: st}
	for _, r := range rows {
		var params map[string]string
		var p *provider
		if dump != nil {
			params = dump[r.Name]
			if params == nil {
				params = map[string]string{}
			}
			if admin {
				p, _ = s.provider(ctx, r.Type)
			}
		}
		out.Remotes = append(out.Remotes, s.view(r, mounted, params, p, admin))
	}
	return out, nil
}

// Get returns one remote.
func (s *Service) Get(ctx context.Context, name string, admin bool) (Remote, error) {
	r, err := s.row(ctx, name)
	if err != nil {
		return Remote{}, err
	}
	if !s.status().Running {
		return s.view(r, nil, nil, nil, admin), nil
	}
	params, err := s.section(ctx, name)
	if err != nil {
		params = map[string]string{}
	}
	var p *provider
	if admin {
		p, _ = s.provider(ctx, r.Type)
	}
	return s.view(r, s.mountedPoints(ctx), params, p, admin), nil
}

func (s *Service) row(ctx context.Context, name string) (row, error) {
	r, ok, err := s.db.get(ctx, name)
	if err != nil {
		return row{}, err
	}
	if !ok {
		return row{}, v1.Errorf(http.StatusNotFound, "there is no remote called "+name)
	}
	return r, nil
}
