package events

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentRegisterUnregisterSourceUpdates(t *testing.T) {
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				ch := Register("stress-user", []string{"stress-source"})
				Unregister("stress-user", ch)
			}
		}
	}()

	for i := 0; i < 200; i++ {
		SendSourceUpdate("stress-source", "tick")
	}
	close(stop)
	time.Sleep(50 * time.Millisecond)
}

func TestConcurrentSSEClientLifecycleRace(t *testing.T) {
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			user := "race-user-" + string(rune('a'+id))
			source := "race-source-" + string(rune('a'+id%2))
			for {
				select {
				case <-stop:
					return
				default:
					ch := Register(user, []string{source, "shared-source"})
					SendSourceUpdate(source, "tick")
					SendSourceUpdate("shared-source", "tick")
					SendToUsers("ping", "msg", []string{user})
					Unregister(user, ch)
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				SendSourceUpdate("shared-source", "broadcast")
				SendToUsers("broadcast", "all", []string{"race-user-a", "race-user-b"})
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
	Shutdown()
	time.Sleep(50 * time.Millisecond)
}

func TestBroadcastReachesEveryRegisteredClient(t *testing.T) {
	a := Register("broadcast-a", nil)
	b := Register("broadcast-b", nil)
	defer Unregister("broadcast-a", a)
	defer Unregister("broadcast-b", b)

	Broadcast("notification", `"for everyone"`)

	for name, ch := range map[string]chan EventMessage{"a": a, "b": b} {
		select {
		case msg := <-ch:
			if msg.Message != `"for everyone"` {
				t.Errorf("client %s got %q", name, msg.Message)
			}
		case <-time.After(time.Second):
			t.Errorf("client %s got nothing", name)
		}
	}
}
