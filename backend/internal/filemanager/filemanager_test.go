package filemanager

import (
	"bytes"
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

// managerApp is a desktop app that can take the file manager's place, or
// says it cannot when unsupported is set.
type managerApp struct {
	plainApp
	on          bool
	words       desktop.Words
	unsupported bool
	fail        error
}

func (a *managerApp) IsFileManager() (bool, error) {
	if a.unsupported {
		return false, errors.ErrUnsupported
	}
	return a.on, nil
}

func (a *managerApp) SetFileManager(on bool, w desktop.Words) error {
	if a.unsupported {
		return errors.ErrUnsupported
	}
	if a.fail != nil {
		return a.fail
	}
	a.on, a.words = on, w
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

func call(t *testing.T, srv *httptest.Server, method string, body any) (int, string, state) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, srv.URL+"/filemanager", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		state
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out.Code, out.state
}

func TestTheSwitchTurnsTheAppIntoTheFileManagerAndBack(t *testing.T) {
	app := &managerApp{}
	srv := server(t, app)
	words := desktop.Words{Open: "In CannonCommander öffnen", Show: "CannonCommander zeigen", Quit: "Beenden"}
	if status, _, got := call(t, srv, http.MethodPut, map[string]any{"on": true, "words": words}); status != http.StatusOK || !got.On {
		t.Fatalf("got %d %+v", status, got)
	}
	if app.words != words {
		t.Fatalf("words %+v", app.words)
	}
	if status, _, got := call(t, srv, http.MethodGet, nil); status != http.StatusOK || !got.On {
		t.Fatalf("got %d %+v", status, got)
	}
	if status, _, got := call(t, srv, http.MethodPut, map[string]any{"on": false}); status != http.StatusOK || got.On {
		t.Fatalf("got %d %+v", status, got)
	}
}

func TestOnlyADesktopAppThatCanHasTheSwitch(t *testing.T) {
	for name, app := range map[string]desktop.App{"container": nil, "older app": plainApp{}, "Mac": &managerApp{unsupported: true}} {
		srv := server(t, app)
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			if status, code, _ := call(t, srv, method, map[string]any{"on": true}); status != http.StatusNotFound || code != "desktopOnly" {
				t.Errorf("%s %s: got %d %s", name, method, status, code)
			}
		}
	}
}

func TestARefusalOfTheComputerIsReported(t *testing.T) {
	srv := server(t, &managerApp{fail: errors.New("access denied")})
	if status, code, _ := call(t, srv, http.MethodPut, map[string]any{"on": true}); status != http.StatusInternalServerError || code != "fileManagerNotSet" {
		t.Fatalf("got %d %s", status, code)
	}
}
