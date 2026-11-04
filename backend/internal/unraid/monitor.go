package unraid

import (
	"context"
	"slices"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// parityEvery spaces out the progress events while a parity run moves.
const parityEvery = 10 * time.Second

// SpinEvent is the payload of unraid.spin.
type SpinEvent struct {
	Asleep  []string `json:"asleep"`
	Changed []string `json:"changed"`
}

// Watch rereads the state every Poll and publishes unraid.spin when a disk
// falls asleep or wakes up, and unraid.parity when a parity run starts,
// moves on or ends. It returns when ctx ends.
func (s *Service) Watch(ctx context.Context, h *hub.Hub) {
	t := time.NewTicker(s.cfg.Poll)
	defer t.Stop()
	var last *State
	var sent time.Time
	for {
		s.fresh()
		st, err := s.State()
		if err != nil {
			logger.Errorf("unraid: %v", err)
		} else if st.Available {
			if last != nil {
				if ev, ok := spinChange(last, st); ok {
					_, _ = h.PublishJSON(EventSpin, "", ev)
				}
				if parityChanged(last.Parity, st.Parity) || st.Parity.Running && time.Since(sent) >= parityEvery && st.Parity.Pos != last.Parity.Pos {
					_, _ = h.PublishJSON(EventParity, "", st.Parity)
					sent = time.Now()
				}
			}
			last = st
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func spinChange(old, cur *State) (SpinEvent, bool) {
	ev := SpinEvent{Asleep: []string{}, Changed: []string{}}
	for _, d := range cur.Disks {
		if d.Spundown {
			ev.Asleep = append(ev.Asleep, d.Name)
		}
		if o, ok := old.Disk(d.Name); ok && o.Spundown != d.Spundown {
			ev.Changed = append(ev.Changed, d.Name)
		}
	}
	slices.Sort(ev.Asleep)
	return ev, len(ev.Changed) > 0
}

func parityChanged(a, b Parity) bool {
	return a.Running != b.Running || a.Paused != b.Paused || a.Kind != b.Kind
}
