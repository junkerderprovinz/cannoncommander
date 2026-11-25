package office

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/dockerengine"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/officefind"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// fakeFinder holds the servers a Docker engine would show, with their
// secrets. Without servers it has no socket.
type fakeFinder struct {
	servers []officefind.Server
	secrets map[string]string
	probed  []string
	here    officefind.Here
}

func (f *fakeFinder) Containers(_ context.Context, here officefind.Here) ([]officefind.Server, error) {
	f.here = here
	if f.servers == nil {
		return nil, officefind.ErrNoDocker
	}
	return f.servers, nil
}

func (f *fakeFinder) Probe(_ context.Context, here officefind.Here, host string) []officefind.Server {
	f.here = here
	f.probed = append(f.probed, host)
	return []officefind.Server{{Kind: system.OfficeCollabora, URL: "https://" + host + ":9980", Reachable: true}}
}

func (f *fakeFinder) Secret(_ context.Context, _ officefind.Here, id string) (officefind.Server, string, error) {
	for _, s := range f.servers {
		if s.ID == id {
			return s, f.secrets[id], nil
		}
	}
	return officefind.Server{}, "", dockerengine.ErrNotFound
}

func (f *fixture) admin(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "tower.lan:8480"
	req.Header.Set("X-User", "boss")
	return f.do(req)
}

func TestWithoutDockerNothingIsFound(t *testing.T) {
	f := newFixture(t)
	rec := f.admin("GET", "/admin/office/found", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"docker":false,"servers":[]}` {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestFoundServersComeWithTheBrowsersHost(t *testing.T) {
	f := newFixture(t)
	f.finder.servers = []officefind.Server{{ID: "e1", Kind: system.OfficeEuroOffice, Name: "euro-office", URL: "http://tower.lan:9900", Reachable: true, Secret: officefind.SecretFound}}
	f.finder.secrets["e1"] = "geheim"
	rec := f.admin("GET", "/admin/office/found", "")
	var got struct {
		Docker  bool
		Servers []officefind.Server
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !got.Docker || len(got.Servers) != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if f.finder.here.Host != "tower.lan" || f.finder.here.Origin != "http://cc.test" {
		t.Fatalf("here %+v", f.finder.here)
	}
	if strings.Contains(rec.Body.String(), "geheim") {
		t.Fatalf("the list carries the secret: %s", rec.Body)
	}
}

func TestConnectTakesTheContainersSecretOnTheServer(t *testing.T) {
	f := newFixture(t)
	f.finder.servers = []officefind.Server{{ID: "e1", Kind: system.OfficeEuroOffice, URL: "http://tower.lan:9900", Reachable: true, Secret: officefind.SecretFound}}
	f.finder.secrets["e1"] = "geheim"
	rec := f.admin("POST", "/admin/office/connect", `{"id":"e1"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(f.connected) != 1 || f.connected[0] != (system.Office{Kind: system.OfficeEuroOffice, URL: "http://tower.lan:9900", Secret: "geheim"}) {
		t.Fatalf("connected %+v", f.connected)
	}
	if strings.Contains(rec.Body.String(), "geheim") {
		t.Fatalf("the answer carries the secret: %s", rec.Body)
	}
	var got struct {
		Result system.IntegrationResult
		Office system.Office
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || !got.Result.OK || !got.Office.SecretSet {
		t.Fatalf("%s", rec.Body)
	}
}

func TestConnectingAContainerThatIsGoneFails(t *testing.T) {
	f := newFixture(t)
	f.finder.servers = []officefind.Server{}
	if rec := f.admin("POST", "/admin/office/connect", `{"id":"gone"}`); rec.Code != 404 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(f.connected) != 0 {
		t.Fatalf("connected %+v", f.connected)
	}
}

func TestAProbeAsksOnlyTheHostItIsGiven(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"tower.lan:9900", "http://tower.lan", "tower.lan/office", ""} {
		if rec := f.admin("POST", "/admin/office/probe", `{"host":"`+bad+`"}`); rec.Code != 400 {
			t.Errorf("%q: %d %s", bad, rec.Code, rec.Body)
		}
	}
	rec := f.admin("POST", "/admin/office/probe", `{"host":"tower.lan"}`)
	if rec.Code != 200 || len(f.finder.probed) != 1 || f.finder.probed[0] != "tower.lan" {
		t.Fatalf("%d %s, probed %v", rec.Code, rec.Body, f.finder.probed)
	}
	if !strings.Contains(rec.Body.String(), `"url":"https://tower.lan:9980"`) {
		t.Fatalf("%s", rec.Body)
	}
}

func TestOnlyAdminsLookForOfficeServers(t *testing.T) {
	f := newFixture(t)
	for _, target := range []string{"/admin/office/found", "/admin/office/probe", "/admin/office/connect"} {
		method := "POST"
		if strings.HasSuffix(target, "found") {
			method = "GET"
		}
		req := httptest.NewRequest(method, target, strings.NewReader(`{}`))
		req.Header.Set("X-User", "jdp")
		if rec := f.do(req); rec.Code != 403 {
			t.Errorf("%s: %d", target, rec.Code)
		}
	}
}
