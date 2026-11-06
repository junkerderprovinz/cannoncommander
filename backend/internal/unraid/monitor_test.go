package unraid

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

func TestWatchPublishesSpinAndParityChanges(t *testing.T) {
	e := newEnv(t)
	h := hub.New(hub.Options{})
	c, _, _ := h.Connect("client", "jdp", 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.svc.Watch(ctx, h)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// The monitor has to know the starting state before it can see a change.
	firstRead(t, e.svc)
	e.writeDisks(map[string]bool{"disk1": true})
	ev := next(t, c.Events, EventSpin)
	var spin SpinEvent
	if err := json.Unmarshal(ev.Data, &spin); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(spin.Changed, []string{"disk1"}) || len(spin.Asleep) != 0 {
		t.Errorf("spin = %+v", spin)
	}

	e.write(e.cfg.Mdstat, "mdResyncAction=check P\nmdResyncPos=10\nmdResync=100\nmdResyncSize=100\n")
	ev = next(t, c.Events, EventParity)
	var p Parity
	if err := json.Unmarshal(ev.Data, &p); err != nil {
		t.Fatal(err)
	}
	if !p.Running || p.Kind != "check" || !e.svc.ParityBusy() {
		t.Errorf("parity = %+v", p)
	}
}

// firstRead waits until Watch has read the state once. Its first round keeps
// that state as the one to compare with, even if a file changes right after.
func firstRead(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		read := s.cached != nil
		s.mu.Unlock()
		if read {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the monitor never read the state")
		}
		time.Sleep(time.Millisecond)
	}
}

func next(t *testing.T, events <-chan hub.Event, typ string) hub.Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == typ {
				return ev
			}
		case <-timeout:
			t.Fatalf("no %s event", typ)
		}
	}
}

func TestWatchTellsWhenADiskOrPoolComesOrGoes(t *testing.T) {
	e := newEnv(t)
	e.writeDiskList(fakeDisks[:3], nil)
	got := make(chan []string, 4)
	e.svc.OnDevices(func(devices []string) { got <- devices })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.svc.Watch(ctx, hub.New(hub.Options{}))
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	firstRead(t, e.svc)
	e.writeDisks(nil)
	if devices := nextDevices(t, got); !slices.Equal(devices, []string{"cache", "disk1", "disk2", "disk3"}) {
		t.Errorf("after a new disk: %v", devices)
	}
	e.writeDiskList(fakeDisks[1:], nil)
	if devices := nextDevices(t, got); !slices.Equal(devices, []string{"disk1", "disk2", "disk3"}) {
		t.Errorf("after the pool went: %v", devices)
	}
}

func nextDevices(t *testing.T, got <-chan []string) []string {
	t.Helper()
	select {
	case d := <-got:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("no change of the disks and pools")
		return nil
	}
}
