// Package officefind looks for office servers next to CannonCommander: in
// the Docker engine when its socket is mounted, otherwise on the ports they
// usually listen on, at a host the admin names. It only reads.
package officefind

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/dockerengine"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// How the JWT secret of an OnlyOffice or Euro-Office container stands.
const (
	// SecretFound means the container's JWT_SECRET can be taken over.
	SecretFound = "found"
	// SecretOff means the server checks no token.
	SecretOff = "off"
	// SecretUnknown means the server made up its own secret, which only
	// its config file inside the container holds.
	SecretUnknown = "unknown"
)

// Whether Collabora accepts WOPI calls from CannonCommander.
const (
	// WOPIOpen means it names no hosts and takes the first one that calls.
	WOPIOpen = "open"
	// WOPIAllowed means its allow list names CannonCommander.
	WOPIAllowed = "allowed"
	// WOPIBlocked means its allow list names other hosts only.
	WOPIBlocked = "blocked"
)

// Server is an office server that was found. It never carries the secret.
type Server struct {
	// ID is the container's, empty for a server a probe found.
	ID   string `json:"id,omitempty"`
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
	// Reachable says that CannonCommander got an answer at URL.
	Reachable bool `json:"reachable"`
	// Secret is one of the Secret constants, for OnlyOffice and Euro-Office
	// in a container.
	Secret string `json:"secret,omitempty"`
	// WOPI is one of the WOPI constants, for Collabora in a container.
	WOPI string `json:"wopi,omitempty"`
	// Allow is the address Collabora's allow list needs when WOPI is
	// blocked.
	Allow string `json:"allow,omitempty"`
}

// Here is how CannonCommander is seen from outside.
type Here struct {
	// Host is the host name or address the browser opened it at.
	Host string
	// HTTPS says the page came over https, where the browser refuses an
	// editor on plain http.
	HTTPS bool
	// Origin is where an office server calls back, scheme://host:port.
	Origin string
}

// Finder looks for office servers. Docker is nil without a socket.
type Finder struct {
	Docker *dockerengine.Engine
	HTTP   *http.Client
}

// unraidWebUI is the label Unraid keeps a template's WebUI address in.
const unraidWebUI = "net.unraid.docker.webui"

type listed struct {
	ID    string `json:"Id"`
	Image string
}

type binding struct {
	HostIP   string `json:"HostIp"`
	HostPort string
}

type endpoint struct {
	IPAddress string
}

type inspect struct {
	ID     string `json:"Id"`
	Name   string
	Config struct {
		Image  string
		Env    []string
		Labels map[string]string
	}
	State struct {
		Running bool
	}
	HostConfig struct {
		NetworkMode string
	}
	NetworkSettings struct {
		Networks map[string]endpoint
		Ports    map[string][]binding
	}
}

func (c inspect) env(key string) (string, bool) {
	for _, kv := range c.Config.Env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

// kindOf tells the suite by its image. Euro-Office is a fork of OnlyOffice
// and its upstream image is also called documentserver, so it goes first.
func kindOf(image string) string {
	image = strings.ToLower(image)
	switch {
	case strings.Contains(image, "euro-office"), strings.Contains(image, "eurooffice"):
		return system.OfficeEuroOffice
	case strings.Contains(image, "onlyoffice"):
		return system.OfficeOnlyOffice
	case strings.Contains(image, "collabora"):
		return system.OfficeCollabora
	}
	return ""
}

// ErrNoDocker means no Docker socket is mounted.
var ErrNoDocker = errors.New("no docker socket")

// Containers lists the office servers among the running containers.
func (f Finder) Containers(ctx context.Context, here Here) ([]Server, error) {
	if f.Docker == nil {
		return nil, ErrNoDocker
	}
	var list []listed
	if err := f.Docker.Do(ctx, http.MethodGet, "/containers/json", nil, &list); err != nil {
		return nil, err
	}
	drivers, err := f.drivers(ctx)
	if err != nil {
		return nil, err
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []Server
	)
	for _, l := range list {
		if kindOf(l.Image) == "" {
			continue
		}
		wg.Go(func() {
			c, err := f.inspect(ctx, l.ID)
			if err != nil || !c.State.Running {
				return
			}
			s := f.server(ctx, here, c, drivers)
			mu.Lock()
			out = append(out, s)
			mu.Unlock()
		})
	}
	wg.Wait()
	slices.SortFunc(out, func(a, b Server) int { return strings.Compare(a.Kind+a.Name, b.Kind+b.Name) })
	return out, nil
}

// Secret returns the container's server and its JWT secret, which is empty
// when it has none to give.
func (f Finder) Secret(ctx context.Context, here Here, id string) (Server, string, error) {
	if f.Docker == nil {
		return Server{}, "", ErrNoDocker
	}
	c, err := f.inspect(ctx, id)
	if err != nil {
		return Server{}, "", err
	}
	if kindOf(c.Config.Image) == "" || !c.State.Running {
		return Server{}, "", errors.New("the container is not a running office server")
	}
	drivers, err := f.drivers(ctx)
	if err != nil {
		return Server{}, "", err
	}
	s := f.server(ctx, here, c, drivers)
	secret := ""
	if s.Secret == SecretFound {
		secret, _ = c.env("JWT_SECRET")
	}
	return s, secret, nil
}

func (f Finder) inspect(ctx context.Context, id string) (inspect, error) {
	var c inspect
	err := f.Docker.Do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil, &c)
	return c, err
}

// drivers maps each network's name to its driver.
func (f Finder) drivers(ctx context.Context) (map[string]string, error) {
	var nets []struct{ Name, Driver string }
	if err := f.Docker.Do(ctx, http.MethodGet, "/networks", nil, &nets); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, n := range nets {
		out[n.Name] = n.Driver
	}
	return out, nil
}

func (f Finder) server(ctx context.Context, here Here, c inspect, drivers map[string]string) Server {
	s := Server{ID: c.ID, Kind: kindOf(c.Config.Image), Name: strings.TrimPrefix(c.Name, "/")}
	if s.Kind == system.OfficeCollabora {
		s.WOPI, s.Allow = wopi(c, here)
	} else {
		s.Secret = SecretUnknown
		if on, _ := c.env("JWT_ENABLED"); strings.EqualFold(on, "false") {
			s.Secret = SecretOff
		} else if v, _ := c.env("JWT_SECRET"); v != "" {
			s.Secret = SecretFound
		}
	}
	s.URL, s.Reachable = f.pick(ctx, here, s.Kind, addresses(here, c, s.Kind, drivers))
	return s
}

type port struct {
	number, scheme string
}

// ports are where the suite listens inside its container.
func ports(c inspect, kind string) []port {
	if kind != system.OfficeCollabora {
		return []port{{"80", "http"}, {"443", "https"}}
	}
	scheme := "https"
	if extra, _ := c.env("extra_params"); strings.Contains(extra, "ssl.enable=false") {
		scheme = "http"
	}
	return []port{{"9980", scheme}}
}

// lanDrivers put a container on the network the browser is on.
var lanDrivers = map[string]bool{"macvlan": true, "ipvlan": true}

var (
	traefikHost = regexp.MustCompile("Host\\(`([^`]+)`\\)")
	webUIPort   = regexp.MustCompile(`\[PORT:(\d+)\]`)
)

// origin is scheme://host:port, without the scheme's own port.
func origin(scheme, host, p string) string {
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(host, p)}
	if (scheme == "http" && p == "80") || (scheme == "https" && p == "443") {
		u.Host = strings.TrimSuffix(u.Host, ":"+p)
	}
	return u.String()
}

// addresses are where the server may answer, best guess first: the address
// a label declares, then a published port on the host the browser used,
// then the container's own address on the LAN and on Docker's networks.
func addresses(here Here, c inspect, kind string, drivers map[string]string) []string {
	var out []string
	add := func(scheme, host, p string) {
		if a := origin(scheme, host, p); host != "" && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	var lan, docker []string
	for name, ep := range c.NetworkSettings.Networks {
		if ep.IPAddress == "" {
			continue
		}
		if lanDrivers[drivers[name]] {
			lan = append(lan, ep.IPAddress)
		} else {
			docker = append(docker, ep.IPAddress)
		}
	}
	slices.Sort(lan)
	slices.Sort(docker)
	host := c.HostConfig.NetworkMode == "host"

	if raw := c.Config.Labels[unraidWebUI]; raw != "" {
		ip := here.Host
		if len(lan) > 0 && !host {
			ip = lan[0]
		}
		raw = strings.ReplaceAll(raw, "[IP]", ip)
		raw = webUIPort.ReplaceAllStringFunc(raw, func(m string) string {
			p := webUIPort.FindStringSubmatch(m)[1]
			if b := c.NetworkSettings.Ports[p+"/tcp"]; len(b) > 0 && b[0].HostPort != "" {
				return b[0].HostPort
			}
			return p
		})
		if u, err := url.Parse(raw); err == nil && (u.Scheme == "http" || u.Scheme == "https") && !strings.ContainsAny(u.Host, "[]") {
			p := u.Port()
			if p == "" {
				p = map[string]string{"http": "80", "https": "443"}[u.Scheme]
			}
			add(u.Scheme, u.Hostname(), p)
		}
	}
	for k, v := range c.Config.Labels {
		if strings.HasPrefix(k, "traefik.http.routers.") && strings.HasSuffix(k, ".rule") {
			for _, m := range traefikHost.FindAllStringSubmatch(v, -1) {
				add("https", m[1], "443")
			}
		}
	}
	for _, p := range ports(c, kind) {
		for _, b := range c.NetworkSettings.Ports[p.number+"/tcp"] {
			add(p.scheme, here.Host, b.HostPort)
		}
	}
	for _, p := range ports(c, kind) {
		if host {
			add(p.scheme, here.Host, p.number)
		}
		for _, ip := range append(lan, docker...) {
			add(p.scheme, ip, p.number)
		}
	}
	return out
}

// pick returns the first address that answers, one over https first when
// the page is on https. When none answers it is the first guess.
func (f Finder) pick(ctx context.Context, here Here, kind string, addrs []string) (string, bool) {
	if len(addrs) == 0 {
		return "", false
	}
	if here.HTTPS {
		plain := func(a string) int {
			if strings.HasPrefix(a, "https:") {
				return 0
			}
			return 1
		}
		slices.SortStableFunc(addrs, func(a, b string) int { return plain(a) - plain(b) })
	}
	answers := make([]bool, len(addrs))
	var wg sync.WaitGroup
	for i, a := range addrs {
		wg.Go(func() { answers[i] = f.answers(ctx, kind, a) })
	}
	wg.Wait()
	for i, ok := range answers {
		if ok {
			return addrs[i], true
		}
	}
	return addrs[0], false
}

// timeout keeps a firewall that drops the packets from holding up the page.
const timeout = 3 * time.Second

// answers reports whether the suite answers at base: the health check of
// OnlyOffice and Euro-Office, the WOPI discovery of Collabora.
func (f Finder) answers(ctx context.Context, kind, base string) bool {
	if kind == system.OfficeCollabora {
		body, ok := f.get(ctx, base+"/hosting/discovery")
		return ok && bytes.Contains(body, []byte("<wopi-discovery"))
	}
	body, ok := f.get(ctx, base+"/healthcheck")
	return ok && strings.TrimSpace(string(body)) == "true"
}

func (f Finder) get(ctx context.Context, addr string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, false
	}
	res, err := f.HTTP.Do(req)
	if err != nil {
		return nil, false
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return body, err == nil && res.StatusCode == http.StatusOK
}

// wopi checks Collabora's allow list against where CannonCommander calls
// from. aliasgroup1, aliasgroup2 and so on hold addresses, the older domain
// a pattern of host names.
func wopi(c inspect, here Here) (string, string) {
	listed, set := false, false
	ours, _ := url.Parse(here.Origin)
	for _, kv := range c.Config.Env {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(k, "aliasgroup") && isDigits(strings.TrimPrefix(k, "aliasgroup")) && v != "":
			set = true
			for _, a := range strings.Split(v, ",") {
				if u, err := url.Parse(strings.TrimSpace(a)); err == nil && ours != nil && sameOrigin(u, ours) {
					listed = true
				}
			}
		case k == "domain" && v != "":
			set = true
			if re, err := regexp.Compile(`^(?:` + v + `)$`); err == nil && ours != nil && re.MatchString(ours.Hostname()) {
				listed = true
			}
		}
	}
	switch {
	case !set:
		return WOPIOpen, ""
	case listed:
		return WOPIAllowed, ""
	}
	return WOPIBlocked, here.Origin
}

func isDigits(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func sameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && originPort(a) == originPort(b)
}

func originPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// probePorts are where the suites listen by default and in the house
// templates: 9900 and 9943 for Euro-Office on Unraid, 35201 on umbrelOS,
// 9980 for Collabora.
var probePorts = []port{
	{"80", "http"}, {"8080", "http"}, {"9900", "http"}, {"9980", "http"},
	{"443", "https"}, {"9943", "https"}, {"9980", "https"}, {"35201", "https"},
}

// Probe asks host on the usual ports what answers there. It talks to that
// one host only, never to the rest of the network.
func (f Finder) Probe(ctx context.Context, here Here, host string) []Server {
	found := make([]Server, len(probePorts))
	var wg sync.WaitGroup
	for i, p := range probePorts {
		wg.Go(func() {
			base := origin(p.scheme, host, p.number)
			for _, kind := range []string{system.OfficeOnlyOffice, system.OfficeCollabora} {
				if f.answers(ctx, kind, base) {
					found[i] = Server{Kind: kind, URL: base, Reachable: true}
					return
				}
			}
		})
	}
	wg.Wait()
	var out []Server
	for _, s := range found {
		if s.URL != "" {
			out = append(out, s)
		}
	}
	return out
}

// ValidHost reports whether s is a bare host name or address, without a
// scheme, port or path.
func ValidHost(s string) error {
	if s == "" || strings.ContainsAny(s, "/?#@ ") {
		return fmt.Errorf("%q: enter a host name or address only", s)
	}
	if ip := net.ParseIP(strings.Trim(s, "[]")); ip != nil {
		return nil
	}
	if strings.Contains(s, ":") {
		return fmt.Errorf("%q: enter the host without a port", s)
	}
	return nil
}
