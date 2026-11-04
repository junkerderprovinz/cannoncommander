// Package office opens office files in OnlyOffice, Euro-Office or
// Collabora. For OnlyOffice and Euro-Office it signs the editor config and
// takes the saved document back through the callback, for Collabora it is
// the WOPI host.
package office

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and the handlers.
func Routes(r *v1.Router) {
	r.Contract("office", contract)
	d := r.Deps()
	bind(r, &service{
		fs:   d.FS,
		keys: &keys{db: d.DB},
		settings: func(ctx context.Context) (system.Office, error) {
			a, err := system.Settings(ctx, d)
			return a.Office, err
		},
		user: func(id uint64) (*users.User, error) {
			u, err := state.GetUserByID(id)
			return &u, err
		},
		base:      publicBase,
		discovery: &discovery{},
		locks:     newLocks(),
		client:    &http.Client{Timeout: 2 * time.Minute},
	})
}

type service struct {
	fs        *ccfs.Resolver
	keys      *keys
	settings  func(context.Context) (system.Office, error)
	user      func(id uint64) (*users.User, error)
	base      func(r *http.Request) string
	discovery *discovery
	locks     *locks
	client    *http.Client
}

func bind(r *v1.Router, s *service) {
	r.Handle("GET /office/config", s.config)
	r.Handle("GET /office/file", s.onlyofficeFile)
	r.Handle("POST /office/callback", s.onlyofficeCallback)
	r.Handle("GET /wopi/files/{id}", s.checkFileInfo)
	r.Handle("POST /wopi/files/{id}", s.lock)
	r.Handle("GET /wopi/files/{id}/contents", s.getFile)
	r.Handle("POST /wopi/files/{id}/contents", s.putFile)
}

// publicBase is the address of /api/v1 as the document server reaches it:
// the configured internal or external address, otherwise the one the
// browser used.
func publicBase(r *http.Request) string {
	api := strings.Trim(settings.Config.Http.BaseURL, "/")
	if api != "" {
		api = "/" + api
	}
	api += "/api/v1"
	for _, configured := range []string{settings.Config.Http.InternalUrl, settings.Config.Http.ExternalUrl} {
		if configured != "" {
			return strings.TrimRight(configured, "/") + api
		}
	}
	scheme, host := "http", r.Host
	if r.TLS != nil {
		scheme = "https"
	}
	if settings.Config.Http.TrustProxyHeaders {
		if h, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Host"), ","); strings.TrimSpace(h) != "" {
			host, scheme = strings.TrimSpace(h), "https"
			if p, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ","); strings.TrimSpace(p) == "http" {
				scheme = "http"
			}
		}
	}
	return scheme + "://" + host + api
}

// Config is what the viewer needs to open one file. For OnlyOffice and
// Euro-Office it is the editor config, signed with the server's JWT secret
// when one is set. For Collabora it is the editor address and the WOPI
// access token with its end in milliseconds.
type Config struct {
	Kind   string         `json:"kind"`
	URL    string         `json:"url"`
	Config map[string]any `json:"config,omitempty"`
	Token  string         `json:"token,omitempty"`
	TTL    int64          `json:"ttl,omitempty"`
}

func (s *service) config(w http.ResponseWriter, r *http.Request) error {
	o, err := s.settings(r.Context())
	if err != nil {
		return err
	}
	if o.Kind == system.OfficeNone || o.URL == "" {
		return v1.Errorf(http.StatusConflict, "no office server is set up")
	}
	u := v1.UserFrom(r)
	loc, info, err := s.file(u, r.URL.Query().Get("path"), ccfs.Download)
	if err != nil {
		return err
	}
	_, werr := s.fs.Check(u, loc.Path, ccfs.Modify)
	// A preview asks for view mode here, because a signed config cannot be
	// changed in the browser.
	g := grant{User: u.ID, Path: loc.Path, Write: werr == nil && r.URL.Query().Get("mode") != "view"}
	token, expires, err := s.keys.sign(r.Context(), g)
	if err != nil {
		return err
	}
	var out Config
	switch o.Kind {
	case system.OfficeCollabora:
		action, err := s.discovery.action(r.Context(), s.client, o.URL, ext(loc.Path), g.Write)
		if err != nil {
			return err
		}
		out = Config{Kind: o.Kind, URL: action + "WOPISrc=" + url.QueryEscape(s.base(r)+"/wopi/files/"+fileID(loc.Path)), Token: token, TTL: expires.UnixMilli()}
	default:
		cfg, err := s.editorConfig(r, o, u, loc, info, g, token)
		if err != nil {
			return err
		}
		out = Config{Kind: o.Kind, URL: o.URL, Config: cfg}
	}
	w.Header().Set("Cache-Control", "no-store")
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// file resolves p for u with perm and makes sure it is a plain file.
func (s *service) file(u *users.User, p string, perm ccfs.Perm) (ccfs.Location, os.FileInfo, error) {
	loc, err := s.fs.Check(u, p, perm)
	if err != nil {
		return ccfs.Location{}, nil, err
	}
	if loc.Virtual() || loc.InArchive() {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "only a file below a source opens in the office server")
	}
	if err := ccfs.Contained(loc); err != nil {
		return ccfs.Location{}, nil, err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return ccfs.Location{}, nil, err
	}
	if !info.Mode().IsRegular() {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "only a file opens in the office server")
	}
	return loc, info, nil
}

// granted checks a token from the document server and the rights its user
// holds now.
func (s *service) granted(ctx context.Context, token string, write bool) (grant, *users.User, ccfs.Location, os.FileInfo, error) {
	g, err := s.keys.verify(ctx, token)
	if err != nil {
		return grant{}, nil, ccfs.Location{}, nil, err
	}
	if write && !g.Write {
		return grant{}, nil, ccfs.Location{}, nil, errBadToken
	}
	u, err := s.user(g.User)
	if err != nil {
		return grant{}, nil, ccfs.Location{}, nil, errBadToken
	}
	// The document server passes the whole file on to the browser, which
	// /fs/raw counts as a download.
	perm := ccfs.Download
	if write {
		perm = ccfs.Modify
	}
	loc, info, err := s.file(u, g.Path, perm)
	return g, u, loc, info, err
}

// save replaces the file with what read delivers. The new content lands
// next to the file first, so a broken transfer never leaves half a
// document, and the file keeps its permissions.
func save(loc ccfs.Location, info os.FileInfo, write func(*os.File) error) error {
	dir, name := filepath.Split(loc.Real)
	tmp, err := os.CreateTemp(dir, "."+name+".*.cc-part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := write(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), loc.Real)
}

func ext(p string) string { return strings.ToLower(strings.TrimPrefix(path.Ext(p), ".")) }

// documentKey changes with every change of the file, which tells the
// document server to drop its cached copy, and is the same for everyone
// who opens it at once, so they edit together.
func documentKey(loc ccfs.Location, info os.FileInfo) string {
	sum := sha256.Sum256([]byte(loc.Path + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\x00" + strconv.FormatInt(info.Size(), 10)))
	return base64.RawURLEncoding.EncodeToString(sum[:15])
}

func version(info os.FileInfo) string {
	return fmt.Sprintf("%d-%d", info.ModTime().UnixNano(), info.Size())
}
