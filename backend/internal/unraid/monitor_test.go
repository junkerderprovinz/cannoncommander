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

	// Give the monitor one round to learn the starting state.
	time.Sleep(3 * e.cfg.Poll)
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
