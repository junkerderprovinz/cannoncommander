package officefind

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/dockerengine"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// engine answers the calls the finder makes, from inspect documents.
type engine struct {
	containers []inspect
	networks   map[string]string
}

func (e *engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch p := r.URL.Path; {
	case r.Method != http.MethodGet:
		w.WriteHeader(http.StatusMethodNotAllowed)
	case p == "/containers/json":
		var list []listed
		for _, c := range e.containers {
			if c.State.Running {
				list = append(list, listed{ID: c.ID, Image: c.Config.Image})
			}
		}
		reply(list)
	case strings.HasPrefix(p, "/containers/") && strings.HasSuffix(p, "/json"):
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/containers/"), "/json")
		for _, c := range e.containers {
			if c.ID == id {
				reply(c)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	case p == "/networks":
		var out []map[string]string
		for name, driver := range e.networks {
			out = append(out, map[string]string{"Name": name, "Driver": driver})
		}
		reply(out)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func container(id, name, image string, env ...string) inspect {
	c := inspect{ID: id, Name: "/" + name}
	c.Config.Image = image
	c.Config.Env = env
	c.Config.Labels = map[string]string{}
	c.State.Running = true
	c.HostConfig.NetworkMode = "bridge"
	c.NetworkSettings.Networks = map[string]endpoint{}
	c.NetworkSettings.Ports = map[string][]binding{}
	return c
}

func (c inspect) on(network, ip string) inspect {
	c.NetworkSettings.Networks[network] = endpoint{IPAddress: ip}
	return c
}

func (c inspect) publish(private, public string) inspect {
	c.NetworkSettings.Ports[private+"/tcp"] = append(c.NetworkSettings.Ports[private+"/tcp"], binding{HostIP: "0.0.0.0", HostPort: public})
	return c
}

func (c inspect) label(k, v string) inspect {
	c.Config.Labels[k] = v
	return c
}

// lan is a set of office servers by the address they answer at. Every
// other address refuses the connection, as a closed port does.
type lan struct {
	mu      sync.Mutex
	servers map[string]*httptest.Server
	asked   []string
}

func newLAN(t *testing.T) *lan {
	l := &lan{servers: map[string]*httptest.Server{}}
	t.Cleanup(func() {
		for _, s := range l.servers {
			s.Close()
		}
	})
	return l
}

func documentServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthcheck" {
			_, _ = w.Write([]byte("true"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func collabora() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hosting/discovery" {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><wopi-discovery><net-zone/></wopi-discovery>`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

// serve answers at addr ("host:port"), over https when tls is set.
func (l *lan) serve(addr string, h http.Handler, tls bool) {
	s := httptest.NewUnstartedServer(h)
	if tls {
		s.StartTLS()
	} else {
		s.Start()
	}
	l.servers[addr] = s
}

func (l *lan) client() *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			l.mu.Lock()
			l.asked = append(l.asked, addr)
			s, ok := l.servers[addr]
			l.mu.Unlock()
			if !ok {
				return nil, &net.OpError{Op: "dial", Net: network, Err: errRefused}
			}
			var d net.Dialer
			return d.DialContext(ctx, "tcp", s.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
}

type refused struct{}

func (refused) Error() string { return "connection refused" }

var errRefused = refused{}

func finder(t *testing.T, e *engine, l *lan) Finder {
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return Finder{Docker: dockerengine.New(srv.Client(), srv.URL), HTTP: l.client()}
}

var tower = Here{Host: "tower.lan", Origin: "http://tower.lan:8480"}

func byKind(t *testing.T, got []Server) map[string]Server {
	t.Helper()
	out := map[string]Server{}
	for _, s := range got {
		out[s.Kind] = s
	}
	return out
}

func TestEachSuiteIsFoundByItsImage(t *testing.T) {
	l := newLAN(t)
	e := &engine{containers: []inspect{
		container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office:latest", "JWT_SECRET=s3"),
		container("o1", "OnlyOffice", "onlyoffice/documentserver:8.3", "JWT_SECRET=s4"),
		container("c1", "collabora", "collabora/code:24.04"),
		container("n1", "nginx", "nginx:alpine"),
	}}
	stopped := container("o2", "old-onlyoffice", "onlyoffice/documentserver")
	stopped.State.Running = false
	e.containers = append(e.containers, stopped)
	got, err := finder(t, e, l).Containers(context.Background(), tower)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("found %d servers, want 3: %+v", len(got), got)
	}
	k := byKind(t, got)
	for kind, name := range map[string]string{system.OfficeEuroOffice: "euro-office", system.OfficeOnlyOffice: "OnlyOffice", system.OfficeCollabora: "collabora"} {
		if k[kind].Name != name || k[kind].ID == "" {
			t.Errorf("%s: got %+v, want the container %s", kind, k[kind], name)
		}
	}
}

func TestTheUpstreamEuroOfficeImageIsNotTakenForOnlyOffice(t *testing.T) {
	e := &engine{containers: []inspect{container("e1", "eo", "ghcr.io/euro-office/documentserver:latest")}}
	got, _ := finder(t, e, newLAN(t)).Containers(context.Background(), tower)
	if len(got) != 1 || got[0].Kind != system.OfficeEuroOffice {
		t.Fatalf("got %+v, want one Euro-Office", got)
	}
}

func TestThePublishedPortOnTheBrowsersHostComesFirst(t *testing.T) {
	l := newLAN(t)
	l.serve("tower.lan:9900", documentServer(), false)
	l.serve("172.17.0.5:80", documentServer(), false)
	e := &engine{
		containers: []inspect{container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office").on("bridge", "172.17.0.5").publish("80", "9900")},
		networks:   map[string]string{"bridge": "bridge"},
	}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if len(got) != 1 || got[0].URL != "http://tower.lan:9900" || !got[0].Reachable {
		t.Fatalf("got %+v, want http://tower.lan:9900 and reachable", got)
	}
}

func TestAnAddressThatDoesNotAnswerGivesWayToTheNext(t *testing.T) {
	l := newLAN(t)
	l.serve("192.168.20.30:80", documentServer(), false)
	e := &engine{
		containers: []inspect{container("o1", "onlyoffice", "onlyoffice/documentserver").on("br0", "192.168.20.30").publish("80", "9900")},
		networks:   map[string]string{"br0": "ipvlan"},
	}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if len(got) != 1 || got[0].URL != "http://192.168.20.30" || !got[0].Reachable {
		t.Fatalf("got %+v, want the LAN address", got)
	}
}

func TestALANAddressComesBeforeADockerOne(t *testing.T) {
	l := newLAN(t)
	l.serve("192.168.20.30:80", documentServer(), false)
	l.serve("172.18.0.4:80", documentServer(), false)
	e := &engine{
		containers: []inspect{container("o1", "onlyoffice", "onlyoffice/documentserver").on("office", "172.18.0.4").on("br0", "192.168.20.30")},
		networks:   map[string]string{"br0": "macvlan", "office": "bridge"},
	}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if got[0].URL != "http://192.168.20.30" {
		t.Fatalf("got %s, want the LAN address", got[0].URL)
	}
}

func TestAServerNobodyReachesIsStillListed(t *testing.T) {
	e := &engine{containers: []inspect{container("o1", "onlyoffice", "onlyoffice/documentserver").publish("80", "9900")}}
	got, _ := finder(t, e, newLAN(t)).Containers(context.Background(), tower)
	if len(got) != 1 || got[0].Reachable || got[0].URL != "http://tower.lan:9900" {
		t.Fatalf("got %+v, want the published address, not reachable", got)
	}
}

func TestUnraidsWebUIGivesTheAddress(t *testing.T) {
	l := newLAN(t)
	l.serve("office.example.com:443", documentServer(), true)
	l.serve("tower.lan:9900", documentServer(), false)
	e := &engine{containers: []inspect{
		container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office").publish("80", "9900").label(unraidWebUI, "https://office.example.com/welcome/"),
	}}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if got[0].URL != "https://office.example.com" {
		t.Fatalf("got %s, want the WebUI's address", got[0].URL)
	}
}

func TestUnraidsWebUIPlaceholdersAreFilledIn(t *testing.T) {
	l := newLAN(t)
	l.serve("tower.lan:9901", documentServer(), false)
	e := &engine{containers: []inspect{
		container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office").publish("80", "9901").label(unraidWebUI, "http://[IP]:[PORT:80]/"),
	}}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if got[0].URL != "http://tower.lan:9901" || !got[0].Reachable {
		t.Fatalf("got %+v, want http://tower.lan:9901", got[0])
	}
}

func TestTraefiksHostRuleGivesTheAddress(t *testing.T) {
	l := newLAN(t)
	l.serve("docs.example.org:443", documentServer(), true)
	e := &engine{containers: []inspect{
		container("o1", "onlyoffice", "onlyoffice/documentserver").label("traefik.http.routers.docs.rule", "Host(`docs.example.org`)"),
	}}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	if got[0].URL != "https://docs.example.org" {
		t.Fatalf("got %s, want the Traefik host", got[0].URL)
	}
}

func TestAPageOnHTTPSPrefersAnHTTPSAddress(t *testing.T) {
	l := newLAN(t)
	l.serve("tower.lan:9900", documentServer(), false)
	l.serve("tower.lan:9943", documentServer(), true)
	e := &engine{containers: []inspect{
		container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office").publish("80", "9900").publish("443", "9943"),
	}}
	here := tower
	here.HTTPS = true
	got, _ := finder(t, e, l).Containers(context.Background(), here)
	if got[0].URL != "https://tower.lan:9943" {
		t.Fatalf("got %s, want the https address", got[0].URL)
	}
}

func TestTheSecretIsOnlyHandedOutWhenAskedFor(t *testing.T) {
	e := &engine{containers: []inspect{container("e1", "euro-office", "ghcr.io/junkerderprovinz/euro-office", "JWT_SECRET=very-secret-value")}}
	f := finder(t, e, newLAN(t))
	got, _ := f.Containers(context.Background(), tower)
	if got[0].Secret != SecretFound {
		t.Fatalf("secret %q, want %q", got[0].Secret, SecretFound)
	}
	out, _ := json.Marshal(got)
	if strings.Contains(string(out), "very-secret-value") {
		t.Fatalf("the list carries the secret: %s", out)
	}
	srv, secret, err := f.Secret(context.Background(), tower, "e1")
	if err != nil || secret != "very-secret-value" || srv.Kind != system.OfficeEuroOffice {
		t.Fatalf("got %+v, %q, %v", srv, secret, err)
	}
}

func TestAServerWithoutJWTNeedsNoSecret(t *testing.T) {
	e := &engine{containers: []inspect{
		container("o1", "off", "onlyoffice/documentserver", "JWT_ENABLED=false", "JWT_SECRET=unused"),
		container("o2", "generated", "onlyoffice/documentserver"),
	}}
	got, _ := finder(t, e, newLAN(t)).Containers(context.Background(), tower)
	want := map[string]string{"off": SecretOff, "generated": SecretUnknown}
	if len(got) != len(want) {
		t.Fatalf("found %d servers, want %d", len(got), len(want))
	}
	for _, s := range got {
		if s.Secret != want[s.Name] {
			t.Errorf("%s: secret %q, want %q", s.Name, s.Secret, want[s.Name])
		}
	}
}

func TestTheSecretOfAContainerThatWentAwayIsNotFound(t *testing.T) {
	_, _, err := finder(t, &engine{}, newLAN(t)).Secret(context.Background(), tower, "gone")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestOnlyAnOfficeContainerHandsOutItsSecret(t *testing.T) {
	e := &engine{containers: []inspect{container("n1", "db", "postgres:17", "POSTGRES_PASSWORD=x")}}
	_, secret, err := finder(t, e, newLAN(t)).Secret(context.Background(), tower, "n1")
	if err == nil || secret != "" {
		t.Fatalf("got %q, %v, want an error", secret, err)
	}
}

func TestCollaboraListensOnHTTPSUnlessSSLIsOff(t *testing.T) {
	l := newLAN(t)
	e := &engine{containers: []inspect{
		container("c1", "tls", "collabora/code").publish("9980", "9980"),
		container("c2", "plain", "collabora/code", "extra_params=--o:ssl.enable=false --o:ssl.termination=true").publish("9980", "9981"),
	}}
	got, _ := finder(t, e, l).Containers(context.Background(), tower)
	urls := map[string]string{}
	for _, s := range got {
		urls[s.Name] = s.URL
	}
	if urls["tls"] != "https://tower.lan:9980" || urls["plain"] != "http://tower.lan:9981" {
		t.Fatalf("got %v", urls)
	}
}

func TestCollaborasAllowListDecidesWhetherItTakesUs(t *testing.T) {
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"open", nil, WOPIOpen},
		{"listed", []string{"aliasgroup1=http://tower.lan:8480,https://cc.example.com"}, WOPIAllowed},
		{"default port", []string{"aliasgroup2=http://TOWER.lan:8480/"}, WOPIAllowed},
		{"elsewhere", []string{"aliasgroup1=https://cloud.example.com:443"}, WOPIBlocked},
		{"domain", []string{`domain=tower\.lan`}, WOPIAllowed},
		{"other domain", []string{`domain=cloud\.example\.com`}, WOPIBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &engine{containers: []inspect{container("c1", "collabora", "collabora/code", tc.env...)}}
			got, _ := finder(t, e, newLAN(t)).Containers(context.Background(), tower)
			if got[0].WOPI != tc.want {
				t.Fatalf("wopi %q, want %q", got[0].WOPI, tc.want)
			}
			if tc.want == WOPIBlocked && got[0].Allow != "http://tower.lan:8480" {
				t.Fatalf("allow %q, want our address", got[0].Allow)
			}
		})
	}
}

func TestAProbeFindsServersOnTheKnownPorts(t *testing.T) {
	l := newLAN(t)
	l.serve("tower.lan:9900", documentServer(), false)
	l.serve("tower.lan:9980", collabora(), true)
	l.serve("tower.lan:8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>a router page answers everything</html>"))
	}), false)
	got := Finder{HTTP: l.client()}.Probe(context.Background(), tower, "tower.lan")
	k := byKind(t, got)
	if len(got) != 2 || k[system.OfficeOnlyOffice].URL != "http://tower.lan:9900" || k[system.OfficeCollabora].URL != "https://tower.lan:9980" {
		t.Fatalf("got %+v", got)
	}
	for _, s := range got {
		if !s.Reachable || s.ID != "" {
			t.Errorf("%+v: a probe finds reachable servers and no container", s)
		}
	}
	for _, a := range l.asked {
		if h, _, _ := net.SplitHostPort(a); h != "tower.lan" {
			t.Errorf("the probe asked %s, not only the host it was given", a)
		}
	}
}

func TestWithoutASocketThereIsNoDocker(t *testing.T) {
	f := Finder{HTTP: http.DefaultClient}
	if _, err := f.Containers(context.Background(), tower); !errors.Is(err, ErrNoDocker) {
		t.Fatalf("got %v", err)
	}
	if _, _, err := f.Secret(context.Background(), tower, "e1"); !errors.Is(err, ErrNoDocker) {
		t.Fatalf("got %v", err)
	}
}
