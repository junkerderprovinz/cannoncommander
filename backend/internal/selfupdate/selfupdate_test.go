//go:build desktop

package selfupdate

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

type updatingApp struct {
	plainApp
	state   desktop.UpdateState
	started int
	refuse  bool
}

func (a *updatingApp) UpdateState() desktop.UpdateState { return a.state }

func (a *updatingApp) UpdateNow() error {
	if a.refuse {
		return errors.ErrUnsupported
	}
	a.started++
	a.state.Busy = true
	return nil
}

func server(t *testing.T, app desktop.App) *httptest.Server {
	t.Helper()
	desktop.Use(app, desktop.Options{})
	t.Cleanup(func() { desktop.Use(nil, desktop.Options{}) })
	r := v1.NewRouter(v1.Deps{}, func(http.ResponseWriter, *http.Request) (*users.User, error) {
		return &users.User{Username: "jdp", Permissions: users.Permissions{Admin: true}}, nil
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
	req, _ := http.NewRequest(method, srv.URL+"/admin/app/update", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestUpdateNowStartsADownloadThatTheStateFollows(t *testing.T) {
	app := &updatingApp{state: desktop.UpdateState{Percent: -1}}
	srv := server(t, app)
	if status, got := call(t, srv, http.MethodGet); status != http.StatusOK || got["busy"] != false || got["percent"] != -1.0 {
		t.Fatalf("before: %d %v", status, got)
	}
	if status, got := call(t, srv, http.MethodPost); status != http.StatusAccepted || got["busy"] != true || app.started != 1 {
		t.Fatalf("start: %d %v, started %d", status, got, app.started)
	}
	app.state = desktop.UpdateState{Percent: 100, Ready: "1.4.0"}
	if status, got := call(t, srv, http.MethodGet); status != http.StatusOK || got["ready"] != "1.4.0" {
		t.Fatalf("after: %d %v", status, got)
	}
}

func TestOnlyADesktopAppOutsideAFlatpakUpdatesItself(t *testing.T) {
	for name, app := range map[string]desktop.App{"no app": nil, "older app": plainApp{}} {
		srv := server(t, app)
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if status, got := call(t, srv, method); status != http.StatusNotFound || got["code"] != "desktopOnly" {
				t.Errorf("%s %s: %d %v", name, method, status, got)
			}
		}
	}
	srv := server(t, &updatingApp{refuse: true})
	if status, got := call(t, srv, http.MethodPost); status != http.StatusNotFound || got["code"] != "desktopOnly" {
		t.Errorf("a Flatpak: %d %v", status, got)
	}
}
