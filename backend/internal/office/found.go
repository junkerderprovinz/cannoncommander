package office

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/dockerengine"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/officefind"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

type finder interface {
	Containers(ctx context.Context, here officefind.Here) ([]officefind.Server, error)
	Probe(ctx context.Context, here officefind.Here, host string) []officefind.Server
	Secret(ctx context.Context, here officefind.Here, id string) (officefind.Server, string, error)
}

// newFinder reads the Docker engine when its socket is mounted. A socket
// mounted later only counts after a restart, as every mount does.
func newFinder() officefind.Finder {
	f := officefind.Finder{HTTP: &http.Client{Timeout: 10 * time.Second}}
	if _, err := os.Stat(dockerengine.Socket); err == nil {
		f.Docker = dockerengine.Dial(dockerengine.Socket)
	}
	return f
}

// here is how the browser and the office server see this server.
func (s *service) here(r *http.Request) officefind.Here {
	scheme, host := browserOrigin(r)
	h := officefind.Here{Host: host, HTTPS: scheme == "https"}
	if name, _, err := net.SplitHostPort(host); err == nil {
		h.Host = name
	}
	h.Host = strings.Trim(h.Host, "[]")
	if u, err := url.Parse(s.base(r)); err == nil {
		h.Origin = u.Scheme + "://" + u.Host
	}
	return h
}

type found struct {
	Docker  bool                `json:"docker"`
	Servers []officefind.Server `json:"servers"`
}

// found lists the office servers in Docker. Without a socket, or one this
// server may not read, the page offers to look at a host instead.
func (s *service) found(w http.ResponseWriter, r *http.Request) error {
	servers, err := s.find.Containers(r.Context(), s.here(r))
	if err != nil {
		if !errors.Is(err, officefind.ErrNoDocker) {
			logger.Errorf("looking for office servers in docker: %v", err)
		}
		return v1.WriteJSON(w, r, http.StatusOK, found{Servers: []officefind.Server{}})
	}
	if servers == nil {
		servers = []officefind.Server{}
	}
	return v1.WriteJSON(w, r, http.StatusOK, found{Docker: true, Servers: servers})
}

func (s *service) probe(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Host string `json:"host"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	host := strings.TrimSpace(in.Host)
	if err := officefind.ValidHost(host); err != nil {
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	servers := s.find.Probe(r.Context(), s.here(r), strings.Trim(host, "[]"))
	if servers == nil {
		servers = []officefind.Server{}
	}
	return v1.WriteJSON(w, r, http.StatusOK, found{Servers: servers})
}

// connectFound sets up a server found in Docker. Its secret goes from the
// container straight into the settings and never to the browser.
func (s *service) connectFound(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ID string `json:"id"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	srv, secret, err := s.find.Secret(r.Context(), s.here(r), in.ID)
	switch {
	case errors.Is(err, dockerengine.ErrNotFound):
		return v1.Errorf(http.StatusNotFound, "the container is gone")
	case err != nil:
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	res, o, err := s.connect(r.Context(), system.Office{Kind: srv.Kind, URL: srv.URL, Secret: secret})
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"result": res, "office": o})
}

// browserOrigin is the scheme and host the browser used, or the ones a
// trusted proxy passed on.
func browserOrigin(r *http.Request) (scheme, host string) {
	scheme, host = "http", r.Host
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
	return scheme, host
}
