package system

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func init() {
	migrate.Register(migrate.Main,
		migrate.SQL("system/001_create",
			`CREATE TABLE app_settings (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
		),
	)
}

// Office kinds. OnlyOffice and Euro-Office share one interface.
const (
	OfficeNone       = "none"
	OfficeOnlyOffice = "onlyoffice"
	OfficeEuroOffice = "eurooffice"
	OfficeCollabora  = "collabora"
)

// Cache modes for mounted remotes, as rclone names them.
const (
	CacheOff    = "off"
	CacheWrites = "writes"
	CacheFull   = "full"
)

// App holds the server wide settings an admin changes on the settings
// pages. Secrets are never sent back, SecretSet and TokenSet say whether
// one is stored.
type App struct {
	Office    Office    `json:"office"`
	ArrowLoop ArrowLoop `json:"arrowloop"`
	Previews  Previews  `json:"previews"`
	WebDAV    bool      `json:"webdav"`
	Remotes   Mounting  `json:"remotes"`
	Updates   Updates   `json:"updates"`
	LogLevel  string    `json:"logLevel"`
	// Terminal lets admins open a shell in a folder, as the user the server
	// runs as. It stays off until an admin switches it on, also on a server
	// without sign in, where everybody is admin.
	Terminal bool `json:"terminal"`
}

// Office is the document server that opens office files.
type Office struct {
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Secret    string `json:"secret,omitempty"`
	SecretSet bool   `json:"secretSet"`
}

// ArrowLoop is the sync scheduler that takes folder pairs as jobs.
type ArrowLoop struct {
	URL      string `json:"url"`
	Token    string `json:"token,omitempty"`
	TokenSet bool   `json:"tokenSet"`
	// Paths maps the start of a path here to the same folder in ArrowLoop's
	// container, as /mnt/user to /data, since both mount the shares where
	// they like.
	Paths map[string]string `json:"paths,omitempty"`
}

// Previews switches the thumbnails and the transcoding for the browser.
type Previews struct {
	Thumbs bool `json:"thumbs"`
	Video  bool `json:"video"`
}

// Mounting says how remotes are mounted.
type Mounting struct {
	Automount bool   `json:"automount"`
	Cache     string `json:"cache"`
	// CacheSize caps the VFS cache of all mounts together, in GB.
	CacheSize int `json:"cacheSize"`
}

// Updates says whether the Info page asks GitHub for a newer release as it
// opens. A container is updated from outside, so nothing here installs one.
type Updates struct {
	AutoCheck bool `json:"autoCheck"`
	// AutoUpdate lets the desktop app put a newer release in place for its
	// next start. The app keeps the switch itself, since the update task of
	// a copy installed for all users reads it without this database. A
	// container reports it off.
	AutoUpdate bool `json:"autoUpdate"`
}

// defaults are the settings of a server where nobody saved any yet. They
// follow the config file, so an existing setup keeps working.
func defaults() App {
	c := settings.Config
	a := App{
		Office:   Office{Kind: OfficeNone},
		Previews: Previews{Thumbs: !c.Server.DisablePreviews},
		WebDAV:   !c.Http.DisableWebDAV,
		Remotes:  Mounting{Automount: true, Cache: CacheWrites, CacheSize: 50},
		Updates:  Updates{AutoCheck: !c.Server.DisableUpdateCheck},
		LogLevel: "info",
	}
	if oo := c.Integrations.OnlyOffice; oo.Url != "" {
		a.Office = Office{Kind: OfficeOnlyOffice, URL: oo.Url, Secret: oo.Secret}
	}
	a.mark()
	return a
}

func (a *App) mark() {
	a.Office.SecretSet = a.Office.Secret != ""
	a.ArrowLoop.TokenSet = a.ArrowLoop.Token != ""
}

// public returns a copy without the secrets.
func (a App) public() App {
	a.mark()
	a.Office.Secret = ""
	a.ArrowLoop.Token = ""
	return a
}

func (a *App) validate() error {
	a.Office.URL = strings.TrimRight(strings.TrimSpace(a.Office.URL), "/")
	a.ArrowLoop.URL = strings.TrimRight(strings.TrimSpace(a.ArrowLoop.URL), "/")
	switch a.Office.Kind {
	case OfficeNone:
	case OfficeOnlyOffice, OfficeEuroOffice, OfficeCollabora:
		if a.Office.URL == "" {
			return fmt.Errorf("the office server needs an address")
		}
	default:
		return fmt.Errorf("office kind must be none, onlyoffice, eurooffice or collabora")
	}
	for here, there := range a.ArrowLoop.Paths {
		if !strings.HasPrefix(here, "/") || !strings.HasPrefix(there, "/") {
			return fmt.Errorf("the ArrowLoop paths have to be full paths")
		}
	}
	for _, u := range []string{a.Office.URL, a.ArrowLoop.URL} {
		if u == "" {
			continue
		}
		if err := checkURL(u); err != nil {
			return err
		}
	}
	switch a.Remotes.Cache {
	case CacheOff, CacheWrites, CacheFull:
	default:
		return fmt.Errorf("cache must be off, writes or full")
	}
	if a.Remotes.CacheSize < 1 || a.Remotes.CacheSize > 100000 {
		return fmt.Errorf("cache size is in GB, between 1 and 100000")
	}
	if _, ok := settings.LogLevels[a.LogLevel]; !ok {
		return fmt.Errorf("log level must be error, info or debug")
	}
	a.mark()
	return nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s: the address starts with http:// or https://", raw)
	}
	return nil
}

// store keeps the settings in the main database. Reads come from memory.
type store struct {
	db *sql.DB

	mu     sync.Mutex
	loaded bool
	saved  bool
	app    App
}

const appKey = "app"

func (s *store) get(ctx context.Context) (App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(ctx); err != nil {
		return App{}, err
	}
	return s.app, nil
}

func (s *store) load(ctx context.Context) error {
	if s.loaded {
		return nil
	}
	app, saved, err := readApp(ctx, s.db)
	if err != nil {
		return err
	}
	s.app, s.saved, s.loaded = app, saved, true
	return nil
}

func readApp(ctx context.Context, db *sql.DB) (App, bool, error) {
	a := defaults()
	if db == nil {
		return a, false, errors.New("no database")
	}
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, appKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	// Decoding over the defaults fills in fields a newer version added.
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return a, false, fmt.Errorf("app settings: %w", err)
	}
	a.mark()
	return a, true, nil
}

// update runs change on a copy of the settings and saves the result when
// change and the validation pass.
func (s *store) update(ctx context.Context, change func(*App) error) (App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(ctx); err != nil {
		return App{}, err
	}
	next := s.app
	if err := change(&next); err != nil {
		return App{}, err
	}
	if err := next.validate(); err != nil {
		return App{}, v1.Errorf(http.StatusBadRequest, err.Error())
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return App{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		appKey, string(raw), time.Now().Unix())
	if err != nil {
		return App{}, err
	}
	s.app, s.saved = next, true
	return next, nil
}

func storeFor(d v1.Deps) *store {
	s, _ := v1.Shared(d, "system.settings", func() (*store, error) { return &store{db: d.DB}, nil })
	return s
}

// Settings returns the server wide settings with their secrets. The remotes
// package reads the mount options here, the ArrowLoop handover its address
// and token.
func Settings(ctx context.Context, d v1.Deps) (App, error) {
	return storeFor(d).get(ctx)
}
