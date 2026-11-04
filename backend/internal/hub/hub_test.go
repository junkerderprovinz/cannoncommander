package hub

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func receive(t *testing.T, c *Client) Event {
	t.Helper()
	select {
	case e, ok := <-c.Events:
		if !ok {
			t.Fatalf("stream of %s closed", c.ID)
		}
		return e
	case <-time.After(time.Second):
		t.Fatalf("no event for %s", c.ID)
	}
	return Event{}
}

func silent(t *testing.T, c *Client) {
	t.Helper()
	select {
	case e := <-c.Events:
		t.Fatalf("%s got unexpected event %+v", c.ID, e)
	default:
	}
}

func TestEveryClientGetsEveryBroadcast(t *testing.T) {
	h := New(Options{})
	a, _, _ := h.Connect("a", "jdp", 0)
	b, _, _ := h.Connect("b", "anna", 0)

	for i := 0; i < 3; i++ {
		h.Publish(Event{Type: "job.progress"})
	}
	for _, c := range []*Client{a, b} {
		for want := uint64(1); want <= 3; want++ {
			if got := receive(t, c).ID; got != want {
				t.Fatalf("%s got id %d, want %d", c.ID, got, want)
			}
		}
	}
}

func TestUserEventsStayWithTheirUser(t *testing.T) {
	h := New(Options{})
	a, _, _ := h.Connect("a", "jdp", 0)
	b, _, _ := h.Connect("b", "anna", 0)

	h.Publish(Event{Type: "prefs.changed", Users: []string{"anna"}})

	if got := receive(t, b).Type; got != "prefs.changed" {
		t.Fatalf("anna got %q", got)
	}
	silent(t, a)
}

func TestTopicEventsReachOnlySubscribers(t *testing.T) {
	h := New(Options{})
	a, _, _ := h.Connect("a", "jdp", 0)
	b, _, _ := h.Connect("b", "jdp", 0)
	h.Subscribe("a", "jdp", "fs:/mnt/user/filme")

	h.Publish(Event{Type: "fs.changed", Topic: "fs:/mnt/user/filme"})

	receive(t, a)
	silent(t, b)
}

func TestReconnectReplaysMissedEvents(t *testing.T) {
	h := New(Options{})
	a, _, _ := h.Connect("a", "jdp", 0)
	first := h.Publish(Event{Type: "job.created"})
	receive(t, a)
	h.Disconnect(a)

	h.Publish(Event{Type: "job.progress"})
	h.Publish(Event{Type: "job.done"})

	_, replay, gap := h.Connect("a", "jdp", first)
	if gap {
		t.Fatal("unexpected gap")
	}
	if len(replay) != 2 || replay[0].Type != "job.progress" || replay[1].Type != "job.done" {
		t.Fatalf("replay = %+v", replay)
	}
}

func TestReplayKeepsSubscriptionsOfTheClientID(t *testing.T) {
	h := New(Options{})
	h.Subscribe("a", "jdp", "fs:/mnt/user")
	a, _, _ := h.Connect("a", "jdp", 0)
	seen := h.Publish(Event{Type: "job.done"})
	receive(t, a)
	h.Disconnect(a)

	h.Publish(Event{Type: "fs.changed", Topic: "fs:/mnt/user"})
	h.Publish(Event{Type: "fs.changed", Topic: "fs:/boot"})

	_, replay, _ := h.Connect("a", "jdp", seen)
	if len(replay) != 1 || replay[0].Topic != "fs:/mnt/user" {
		t.Fatalf("replay = %+v", replay)
	}
}

func TestOldLastIDReportsGap(t *testing.T) {
	h := New(Options{History: 2})
	for i := 0; i < 5; i++ {
		h.Publish(Event{Type: "job.progress"})
	}
	_, replay, gap := h.Connect("a", "jdp", 1)
	if !gap {
		t.Fatal("want gap when the history does not reach back far enough")
	}
	if len(replay) != 2 || replay[0].ID != 4 {
		t.Fatalf("replay = %+v", replay)
	}
}

func TestLastIDFromBeforeRestartReportsGap(t *testing.T) {
	h := New(Options{})
	h.Publish(Event{Type: "job.done"})
	if _, _, gap := h.Connect("a", "jdp", 900); !gap {
		t.Fatal("want gap for an id the hub never issued")
	}
}

func TestSlowClientIsDroppedNotWaitedFor(t *testing.T) {
	h := New(Options{Buffer: 2})
	slow, _, _ := h.Connect("slow", "jdp", 0)
	fast, _, _ := h.Connect("fast", "jdp", 0)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			h.Publish(Event{Type: "job.progress"})
			receive(t, fast)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish blocked on a slow client")
	}

	n := 0
	for range slow.Events {
		n++
	}
	if n != 2 {
		t.Fatalf("slow client kept %d events, want the 2 that fit", n)
	}
}

func TestSubscriptionsOfAnotherUserAreNotShared(t *testing.T) {
	h := New(Options{})
	h.Subscribe("a", "jdp", "fs:/mnt/user/backups")
	c, _, _ := h.Connect("a", "anna", 0)

	h.Publish(Event{Type: "fs.changed", Topic: "fs:/mnt/user/backups"})

	silent(t, c)
}

func TestOnTopicFollowsFirstAndLastSubscriber(t *testing.T) {
	var mu sync.Mutex
	var log []string
	h := New(Options{OnTopic: func(topic string, active bool) {
		mu.Lock()
		defer mu.Unlock()
		log = append(log, fmt.Sprintf("%s=%v", topic, active))
	}})

	h.Subscribe("a", "jdp", "fs:/x")
	h.Subscribe("b", "jdp", "fs:/x")
	h.Unsubscribe("a", "fs:/x")
	h.Unsubscribe("b", "fs:/x")
	h.SetTopics("a", "jdp", []string{"fs:/y"})
	h.SetTopics("a", "jdp", []string{"fs:/z"})

	want := []string{"fs:/x=true", "fs:/x=false", "fs:/y=true", "fs:/y=false", "fs:/z=true"}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(log) != fmt.Sprint(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}
}

func TestLingeringSubscriptionsExpire(t *testing.T) {
	released := make(chan string, 1)
	h := New(Options{Linger: 20 * time.Millisecond, OnTopic: func(topic string, active bool) {
		if !active {
			released <- topic
		}
	}})
	c, _, _ := h.Connect("a", "jdp", 0)
	h.Subscribe("a", "jdp", "fs:/mnt/user")
	h.Disconnect(c)

	select {
	case topic := <-released:
		if topic != "fs:/mnt/user" {
			t.Fatalf("released %q", topic)
		}
	case <-time.After(time.Second):
		t.Fatal("subscription never expired")
	}
	if got := h.Topics("a"); got != nil {
		t.Fatalf("topics = %v", got)
	}
}

func TestConcurrentUse(t *testing.T) {
	h := New(Options{Buffer: 4, Linger: time.Millisecond, OnTopic: func(string, bool) {}})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("c%d", i)
			for j := 0; j < 200; j++ {
				c, _, _ := h.Connect(id, "jdp", uint64(j))
				h.Subscribe(id, "jdp", fmt.Sprintf("fs:/%d", j%3))
				h.Publish(Event{Type: "fs.changed", Topic: fmt.Sprintf("fs:/%d", j%3)})
				h.Disconnect(c)
			}
		}(i)
	}
	wg.Wait()
	h.Close()
}
