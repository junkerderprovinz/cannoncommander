// Package stream serves the hub as Server-Sent Events and lets a client
// choose which folders it watches.
package stream

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
)

//go:embed openapi.yaml
var contract []byte

const (
	maxClientID   = 64
	maxWatched    = 64
	heartbeatTick = 25 * time.Second
)

// Routes registers the event stream.
func Routes(r *v1.Router) {
	r.Contract("stream", contract)
	s := server{hub: r.Deps().Hub, fs: r.Deps().FS, heartbeat: heartbeatTick}
	r.Handle("GET /events", s.events)
	r.Handle("PUT /events/watch", s.watch)
}

type server struct {
	hub       *hub.Hub
	fs        *ccfs.Resolver
	heartbeat time.Duration
}

func (s server) events(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		// Without an id the stream still gets every event that is not tied
		// to watched folders.
		clientID = "anonymous-" + rand.Text()
	} else if _, err := clientIDFrom(clientID); err != nil {
		return err
	}
	lastID, err := lastEventID(r)
	if err != nil {
		return err
	}
	rc := http.NewResponseController(w)
	// The server's write timeout would cut the stream off.
	_ = rc.SetWriteDeadline(time.Time{})

	client, replay, gap := s.hub.Connect(clientID, u.Username, lastID)
	defer s.hub.Disconnect(client)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
		return nil
	}
	if gap {
		resync := hub.Event{ID: s.hub.LastID(), Type: hub.TypeResync, Time: time.Now()}
		if writeEvent(w, resync, true) != nil {
			return nil
		}
	}
	for _, e := range replay {
		if writeEvent(w, watch.ForUser(s.fs, u, e), true) != nil {
			return nil
		}
	}
	if rc.Flush() != nil {
		return nil
	}

	tick := time.NewTicker(s.heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case e, ok := <-client.Events:
			if !ok {
				return nil
			}
			if writeEvent(w, watch.ForUser(s.fs, u, e), true) != nil {
				return nil
			}
		case now := <-tick.C:
			if writeEvent(w, hub.Event{Type: hub.TypeHeartbeat, Time: now}, false) != nil {
				return nil
			}
		}
		if rc.Flush() != nil {
			return nil
		}
	}
}

type watchRequest struct {
	ClientID string   `json:"clientId"`
	Paths    []string `json:"paths"`
}

func (s server) watch(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req watchRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	clientID, err := clientIDFrom(req.ClientID)
	if err != nil {
		return err
	}
	if len(req.Paths) > maxWatched {
		return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("at most %d folders can be watched", maxWatched))
	}
	out := watchRequest{ClientID: clientID, Paths: []string{}}
	topics := make([]string, 0, len(req.Paths))
	// A tab can still show a folder that was deleted or closed to the user
	// since. It drops out of the answer instead of failing the other tabs.
	for _, p := range req.Paths {
		loc, err := s.fs.Reach(u, p, ccfs.View)
		if err != nil {
			continue
		}
		if fi, err := os.Stat(loc.Real); err != nil || !fi.IsDir() {
			continue
		}
		out.Paths = append(out.Paths, loc.Path)
		topics = append(topics, hub.FolderTopic(loc.Path))
	}
	s.hub.SetTopics(clientID, u.Username, topics)
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// writeEvent writes one SSE frame. Heartbeats go without an id, so they do
// not move the client's Last-Event-ID.
func writeEvent(w http.ResponseWriter, e hub.Event, withID bool) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if withID {
		if _, err := fmt.Fprintf(w, "id: %d\n", e.ID); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
	return err
}

func clientIDFrom(s string) (string, error) {
	if s == "" {
		return "", v1.Errorf(http.StatusBadRequest, "clientId is required")
	}
	if len(s) > maxClientID {
		return "", v1.Errorf(http.StatusBadRequest, "clientId is too long")
	}
	return s, nil
}

func lastEventID(r *http.Request) (uint64, error) {
	s := r.Header.Get("Last-Event-ID")
	if s == "" {
		s = r.URL.Query().Get("lastEventId")
	}
	if s == "" {
		return 0, nil
	}
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, v1.Errorf(http.StatusBadRequest, "Last-Event-ID must be a number")
	}
	return id, nil
}
