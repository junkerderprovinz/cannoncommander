// Package hub fans server events out to every connected client. Each event
// gets a rising ID and stays in a ring buffer, so a client that reconnects
// with Last-Event-ID receives what it missed.
package hub

import (
	"encoding/json"
	"slices"
	"sync"
	"time"
)

// Event types every package may use. Packages define their own types next
// to the code that publishes them.
const (
	TypeHeartbeat = "heartbeat"
	TypeResync    = "resync"
	TypeNotify    = "notify"
)

// Event is one message on the stream. An empty Topic reaches every client,
// otherwise only clients subscribed to that topic. An empty Users list
// reaches every user.
type Event struct {
	ID    uint64          `json:"id"`
	Type  string          `json:"type"`
	Topic string          `json:"topic,omitempty"`
	Time  time.Time       `json:"time"`
	Data  json.RawMessage `json:"data,omitempty"`
	Users []string        `json:"-"`
}

// Client is one open stream. Events arrives closed when the hub drops the
// client, either on Disconnect or because the client fell too far behind.
type Client struct {
	ID     string
	User   string
	Events <-chan Event

	ch chan Event
}

type subscription struct {
	user    string
	topics  map[string]struct{}
	clients int
	expiry  *time.Timer
}

// Options tunes a hub. Zero values pick the defaults.
type Options struct {
	// History is how many events are kept for replay.
	History int
	// Buffer is the per client queue length.
	Buffer int
	// Linger keeps the subscriptions of a vanished client so a quick
	// reconnect does not have to subscribe again.
	Linger time.Duration
	// OnTopic is told when a topic gains its first subscriber or loses its
	// last one. The watcher uses it to open and close folder watches. Calls
	// arrive in order, one at a time, and must not subscribe or unsubscribe.
	OnTopic func(topic string, active bool)
}

type topicChange struct {
	topic  string
	active bool
}

// Hub is safe for concurrent use.
type Hub struct {
	opts Options

	mu      sync.Mutex
	nextID  uint64
	ring    []Event
	head    int
	clients map[*Client]struct{}
	subs    map[string]*subscription
	topics  map[string]int
	pending []topicChange

	// notifyMu is taken before mu is released, so topic changes reach
	// OnTopic in the order they happened.
	notifyMu sync.Mutex
}

// New returns an empty hub.
func New(opts Options) *Hub {
	if opts.History <= 0 {
		opts.History = 1024
	}
	if opts.Buffer <= 0 {
		opts.Buffer = 256
	}
	if opts.Linger <= 0 {
		opts.Linger = 2 * time.Minute
	}
	return &Hub{
		opts:    opts,
		clients: map[*Client]struct{}{},
		subs:    map[string]*subscription{},
		topics:  map[string]int{},
	}
}

// Publish stamps the event with the next ID and hands it to every client
// that should see it. It never blocks: a client whose queue is full is
// dropped and catches up through Last-Event-ID on reconnect.
func (h *Hub) Publish(e Event) uint64 {
	h.mu.Lock()
	defer h.unlock()
	h.nextID++
	e.ID = h.nextID
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	h.remember(e)
	var lagging []*Client
	for c := range h.clients {
		if !h.wants(c, e) {
			continue
		}
		select {
		case c.ch <- e:
		default:
			lagging = append(lagging, c)
		}
	}
	for _, c := range lagging {
		h.drop(c)
	}
	return e.ID
}

// PublishJSON marshals data and publishes it as an event of the given type.
func (h *Hub) PublishJSON(typ, topic string, data any, users ...string) (uint64, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	return h.Publish(Event{Type: typ, Topic: topic, Data: raw, Users: users}), nil
}

// Connect registers a stream for clientID. When lastID is set, the events
// after it are returned for replay. gap reports that some of them already
// fell out of the history, so the client has to reload its state.
func (h *Hub) Connect(clientID, user string, lastID uint64) (c *Client, replay []Event, gap bool) {
	h.mu.Lock()
	defer h.unlock()
	ch := make(chan Event, h.opts.Buffer)
	c = &Client{ID: clientID, User: user, Events: ch, ch: ch}
	h.clients[c] = struct{}{}
	s := h.subscription(clientID, user)
	s.clients++
	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	if lastID == 0 || lastID == h.nextID {
		return c, nil, false
	}
	// An ID from the future comes from before a server restart.
	if lastID > h.nextID {
		return c, nil, true
	}
	history := h.history()
	if len(history) == 0 || history[0].ID > lastID+1 {
		gap = true
	}
	for _, e := range history {
		if e.ID > lastID && h.wants(c, e) {
			replay = append(replay, e)
		}
	}
	return c, replay, gap
}

// Disconnect closes the client's stream. Its subscriptions linger for a
// while in case the same clientID comes back.
func (h *Hub) Disconnect(c *Client) {
	h.mu.Lock()
	defer h.unlock()
	h.drop(c)
}

// Subscribe adds topics for clientID. The client does not have to be
// connected yet.
func (h *Hub) Subscribe(clientID, user string, topics ...string) {
	h.mu.Lock()
	defer h.unlock()
	s := h.subscription(clientID, user)
	for _, t := range topics {
		if _, ok := s.topics[t]; ok {
			continue
		}
		s.topics[t] = struct{}{}
		h.ref(t, 1)
	}
	if s.clients == 0 {
		h.expireLater(clientID, s)
	}
}

// Unsubscribe removes topics for clientID.
func (h *Hub) Unsubscribe(clientID string, topics ...string) {
	h.mu.Lock()
	defer h.unlock()
	s, ok := h.subs[clientID]
	if !ok {
		return
	}
	for _, t := range topics {
		if _, ok := s.topics[t]; ok {
			delete(s.topics, t)
			h.ref(t, -1)
		}
	}
}

// SetTopics replaces the topics of clientID with the given set.
func (h *Hub) SetTopics(clientID, user string, topics []string) {
	h.mu.Lock()
	s := h.subscription(clientID, user)
	var gone []string
	for t := range s.topics {
		if !slices.Contains(topics, t) {
			gone = append(gone, t)
		}
	}
	h.unlock()
	h.Unsubscribe(clientID, gone...)
	h.Subscribe(clientID, user, topics...)
}

// Topics returns the subscribed topics of clientID in sorted order.
func (h *Hub) Topics(clientID string) []string {
	h.mu.Lock()
	defer h.unlock()
	s, ok := h.subs[clientID]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(s.topics))
	for t := range s.topics {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// LastID returns the ID of the newest event.
func (h *Hub) LastID() uint64 {
	h.mu.Lock()
	defer h.unlock()
	return h.nextID
}

// Close drops every client, for server shutdown.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.unlock()
	for c := range h.clients {
		h.drop(c)
	}
}

func (h *Hub) wants(c *Client, e Event) bool {
	if len(e.Users) > 0 && !slices.Contains(e.Users, c.User) {
		return false
	}
	if e.Topic == "" {
		return true
	}
	s, ok := h.subs[c.ID]
	if !ok || s.user != c.User {
		return false
	}
	_, ok = s.topics[e.Topic]
	return ok
}

func (h *Hub) drop(c *Client) {
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	close(c.ch)
	if s, ok := h.subs[c.ID]; ok {
		s.clients--
		if s.clients == 0 {
			h.expireLater(c.ID, s)
		}
	}
}

// subscription returns the record for clientID. A clientID that shows up
// under another user starts from scratch, so one user cannot read another
// user's topics by guessing the ID.
func (h *Hub) subscription(clientID, user string) *subscription {
	if s, ok := h.subs[clientID]; ok {
		if s.user == user {
			return s
		}
		h.forget(clientID, s)
	}
	s := &subscription{user: user, topics: map[string]struct{}{}}
	h.subs[clientID] = s
	return s
}

func (h *Hub) expireLater(clientID string, s *subscription) {
	if s.expiry != nil {
		s.expiry.Stop()
	}
	s.expiry = time.AfterFunc(h.opts.Linger, func() {
		h.mu.Lock()
		defer h.unlock()
		if cur, ok := h.subs[clientID]; ok && cur == s && s.clients == 0 {
			h.forget(clientID, s)
		}
	})
}

func (h *Hub) forget(clientID string, s *subscription) {
	if s.expiry != nil {
		s.expiry.Stop()
	}
	for t := range s.topics {
		h.ref(t, -1)
	}
	delete(h.subs, clientID)
}

func (h *Hub) ref(topic string, delta int) {
	before := h.topics[topic]
	after := before + delta
	if after <= 0 {
		delete(h.topics, topic)
	} else {
		h.topics[topic] = after
	}
	if h.opts.OnTopic == nil {
		return
	}
	switch {
	case before == 0 && after > 0:
		h.pending = append(h.pending, topicChange{topic, true})
	case before > 0 && after <= 0:
		h.pending = append(h.pending, topicChange{topic, false})
	}
}

// unlock releases mu and then reports the topic changes made under it.
func (h *Hub) unlock() {
	changes := h.pending
	h.pending = nil
	if len(changes) == 0 {
		h.mu.Unlock()
		return
	}
	h.notifyMu.Lock()
	h.mu.Unlock()
	defer h.notifyMu.Unlock()
	for _, c := range changes {
		h.opts.OnTopic(c.topic, c.active)
	}
}

func (h *Hub) remember(e Event) {
	if len(h.ring) < h.opts.History {
		h.ring = append(h.ring, e)
		return
	}
	h.ring[h.head] = e
	h.head = (h.head + 1) % len(h.ring)
}

func (h *Hub) history() []Event {
	if len(h.ring) < h.opts.History {
		return h.ring
	}
	out := make([]Event, 0, len(h.ring))
	out = append(out, h.ring[h.head:]...)
	return append(out, h.ring[:h.head]...)
}
