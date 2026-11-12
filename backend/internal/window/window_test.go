//go:build desktop

package window

import (
	"bytes"
	"encoding/json"
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

type windowApp struct {
	plainApp
	maximised, minimised, closed bool
}

func (a *windowApp) WindowMaximised() bool  { return a.maximised }
func (a *windowApp) MaximiseWindow(on bool) { a.maximised = on }
func (a *windowApp) MinimiseWindow()        { a.minimised = true }
func (a *windowApp) CloseWindow()           { a.closed = true }

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

func call(t *testing.T, srv *httptest.Server, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var in bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&in).Encode(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, &in)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestTheButtonsWorkTheWindow(t *testing.T) {
	app := &windowApp{}
	srv := server(t, app)
	if status, _ := call(t, srv, http.MethodPut, "/window", state{Maximised: true}); status != http.StatusNoContent || !app.maximised {
		t.Fatalf("maximise: %d %+v", status, app)
	}
	if status, got := call(t, srv, http.MethodGet, "/window", nil); status != http.StatusOK || got["maximised"] != true {
		t.Fatalf("state: %d %v", status, got)
	}
	if status, _ := call(t, srv, http.MethodPut, "/window", state{}); status != http.StatusNoContent || app.maximised {
		t.Fatalf("restore: %d %+v", status, app)
	}
	if status, _ := call(t, srv, http.MethodPost, "/window/minimise", nil); status != http.StatusNoContent || !app.minimised {
		t.Fatalf("minimise: %d %+v", status, app)
	}
	if status, _ := call(t, srv, http.MethodPost, "/window/close", nil); status != http.StatusNoContent || !app.closed {
		t.Fatalf("close: %d %+v", status, app)
	}
}

func TestWithoutAWindowTheButtonsAreDesktopOnly(t *testing.T) {
	for name, app := range map[string]desktop.App{"no app": nil, "older app": plainApp{}} {
		srv := server(t, app)
		for _, op := range []struct{ method, path string }{
			{http.MethodGet, "/window"},
			{http.MethodPut, "/window"},
			{http.MethodPost, "/window/minimise"},
			{http.MethodPost, "/window/close"},
		} {
			if status, got := call(t, srv, op.method, op.path, state{}); status != http.StatusNotFound || got["code"] != "desktopOnly" {
				t.Errorf("%s %s %s: %d %v", name, op.method, op.path, status, got)
			}
		}
	}
}
