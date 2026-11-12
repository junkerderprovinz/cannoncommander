//go:build desktop

package modules

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

type plainApp struct{}

func (plainApp) AutoUpdate() bool         { return true }
func (plainApp) SetAutoUpdate(bool) error { return nil }
func (plainApp) UpdateReady() string      { return "" }
func (plainApp) Restart()                 {}

type modulesApp struct {
	plainApp
	state   desktop.ModulesState
	fetched int
	refuse  bool
}

func (a *modulesApp) ModulesState() desktop.ModulesState { return a.state }

func (a *modulesApp) FetchModules() error {
	if a.refuse {
		return errors.ErrUnsupported
	}
	a.fetched++
	a.state.Running = true
	return nil
}

func server(t *testing.T, app desktop.App) *httptest.Server {
	t.Helper()
	desktop.Use(app, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
	r := v1.NewRouter(v1.Deps{}, func(http.ResponseWriter, *http.Request) (*users.User, error) {
		return &users.User{Username: "jdp"}, nil
	})
	Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, srv *httptest.Server, method string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+"/modules", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestTheButtonStartsAFetchThatTheStateFollows(t *testing.T) {
	app := &modulesApp{state: desktop.ModulesState{Offered: true}}
	srv := server(t, app)
	if status, got := call(t, srv, http.MethodGet); status != http.StatusOK || got["offered"] != true || got["running"] != false {
		t.Fatalf("before: %d %v", status, got)
	}
	if status, got := call(t, srv, http.MethodPost); status != http.StatusAccepted || got["running"] != true || app.fetched != 1 {
		t.Fatalf("fetch: %d %v, fetched %d", status, got, app.fetched)
	}
	app.state = desktop.ModulesState{Offered: true, Fraction: 1, Ready: true}
	if status, got := call(t, srv, http.MethodGet); status != http.StatusOK || got["ready"] != true || got["fraction"] != 1.0 {
		t.Fatalf("after: %d %v", status, got)
	}
}

func TestOnlyADesktopAppThatFetchesHasTheModules(t *testing.T) {
	for name, app := range map[string]desktop.App{"no app": nil, "older app": plainApp{}} {
		srv := server(t, app)
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if status, got := call(t, srv, method); status != http.StatusNotFound || got["code"] != "desktopOnly" {
				t.Errorf("%s %s: %d %v", name, method, status, got)
			}
		}
	}
	srv := server(t, &modulesApp{refuse: true})
	if status, got := call(t, srv, http.MethodPost); status != http.StatusNotFound || got["code"] != "desktopOnly" {
		t.Errorf("a Mac bundle: %d %v", status, got)
	}
}
