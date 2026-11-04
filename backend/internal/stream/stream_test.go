package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

type frame struct {
	id, event, data string
}

func setup(t *testing.T) (*httptest.Server, *hub.Hub) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"filme", "fotos"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(hub.Options{})
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		u := &users.User{BackendScopes: []users.BackendScope{{Path: root, Scope: "/fotos", Permissions: users.SourceFilePermissions{View: true}}}}
		u.Username = "anna"
		return u, nil
	}
	r := v1.NewRouter(v1.Deps{FS: fs, Hub: h}, auth)
	Routes(r)
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, h
}

func open(t *testing.T, srv *httptest.Server, clientID, lastID string) <-chan frame {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events?clientId="+clientID, nil)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream answered %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	frames := make(chan frame, 16)
	go func() {
		defer resp.Body.Close()
		defer close(frames)
		s := bufio.NewScanner(resp.Body)
		var f frame
		for s.Scan() {
			line := s.Text()
			switch {
			case line == "":
				if f.event != "" {
					frames <- f
				}
				f = frame{}
			case strings.HasPrefix(line, "id: "):
				f.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return frames
}

// next returns the next frame that is not a connection marker.
func next(t *testing.T, frames <-chan frame) frame {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				t.Fatal("stream ended")
			}
			if f.event == "test.marker" {
				continue
			}
			return f
		case <-timeout:
			t.Fatal("no frame")
		}
	}
}

// waitConnected publishes markers until the stream shows one, because the
// client connects to the hub only after the request has arrived.
func waitConnected(t *testing.T, h *hub.Hub, frames <-chan frame) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.Publish(hub.Event{Type: "test.marker"})
		select {
		case <-frames:
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("stream never connected")
}

func TestStreamDeliversEventsWithIDs(t *testing.T) {
	srv, h := setup(t)
	frames := open(t, srv, "tab1", "")
	waitConnected(t, h, frames)

	id := h.Publish(hub.Event{Type: "job.created", Data: json.RawMessage(`{"id":"j1"}`)})

	f := next(t, frames)
	if f.event != "job.created" || f.id == "" {
		t.Fatalf("frame = %+v", f)
	}
	var e hub.Event
	if err := json.Unmarshal([]byte(f.data), &e); err != nil || e.ID != id || string(e.Data) != `{"id":"j1"}` {
		t.Fatalf("data = %s, %v", f.data, err)
	}
}

func TestReconnectReplaysOrResyncs(t *testing.T) {
	srv, h := setup(t)
	seen := h.Publish(hub.Event{Type: "job.created"})
	h.Publish(hub.Event{Type: "job.progress"})
	h.Publish(hub.Event{Type: "job.done"})

	frames := open(t, srv, "tab1", jsonID(seen))
	if f := next(t, frames); f.event != "job.progress" {
		t.Fatalf("first replayed frame = %+v", f)
	}
	if f := next(t, frames); f.event != "job.done" {
		t.Fatalf("second replayed frame = %+v", f)
	}

	frames = open(t, srv, "tab2", "99999")
	if f := next(t, frames); f.event != "resync" {
		t.Fatalf("stale id gave %+v", f)
	}
}

func TestWatchLimitsFolderEventsToWatchedFolders(t *testing.T) {
	srv, h := setup(t)
	frames := open(t, srv, "tab1", "")
	waitConnected(t, h, frames)

	body := strings.NewReader(`{"clientId":"tab1","paths":["/mnt/user/fotos/"]}`)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/events/watch", body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var got watchRequest
	_ = json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(got.Paths) != 1 || got.Paths[0] != "/mnt/user/fotos" {
		t.Fatalf("watch answered %d %+v", resp.StatusCode, got)
	}

	h.Publish(hub.Event{Type: "fs.changed", Topic: hub.FolderTopic("/mnt/user/filme")})
	h.Publish(hub.Event{Type: "fs.changed", Topic: hub.FolderTopic("/mnt/user/fotos")})

	f := next(t, frames)
	if !strings.Contains(f.data, `"topic":"fs:/mnt/user/fotos"`) {
		t.Fatalf("frame = %+v", f)
	}

	// A forbidden and a missing folder drop out without taking the allowed one along.
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/events/watch", strings.NewReader(`{"clientId":"tab1","paths":["/mnt/user/filme","/mnt/user/fotos/weg","/mnt/user/fotos"]}`))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got = watchRequest{}
	_ = json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(got.Paths) != 1 || got.Paths[0] != "/mnt/user/fotos" {
		t.Fatalf("watch answered %d %+v", resp.StatusCode, got)
	}
	h.Publish(hub.Event{Type: "fs.changed", Topic: hub.FolderTopic("/mnt/user/fotos")})
	if f := next(t, frames); !strings.Contains(f.data, `"topic":"fs:/mnt/user/fotos"`) {
		t.Fatalf("frame = %+v", f)
	}
}

func TestStreamWorksWithoutAClientID(t *testing.T) {
	srv, h := setup(t)
	frames := open(t, srv, "", "")
	waitConnected(t, h, frames)
	h.Publish(hub.Event{Type: "job.done"})
	if f := next(t, frames); f.event != "job.done" {
		t.Fatalf("frame = %+v", f)
	}

	resp, err := http.Get(srv.URL + "/events?clientId=" + strings.Repeat("x", 65))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("long clientId: status %d", resp.StatusCode)
	}
}

func jsonID(id uint64) string {
	b, _ := json.Marshal(id)
	return string(b)
}
