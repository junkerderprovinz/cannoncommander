package mcpapi

import (
	"sync"
	"time"
)

// window counts hits per key over a rolling window. For key ids the map is
// bounded by the keys used in this process, and a key whose window ran empty
// drops out of it. Keyed by client address, maxKeys bounds it instead.
type window struct {
	mu      sync.Mutex
	span    time.Duration
	max     int
	maxKeys int
	hits    map[string][]hit
}

// hit is one use of the window, weighing n.
type hit struct {
	at time.Time
	n  int
}

func newWindow(span time.Duration, max int) *window {
	return &window{span: span, max: max, hits: map[string][]hit{}}
}

// allow records a hit of weight 1 and reports whether it fit. When it did not,
// the second value is how long until the oldest hit falls out.
func (w *window) allow(key string, now time.Time) (bool, time.Duration) {
	return w.take(key, 1, now)
}

// take records a hit of weight n when all of it still fits, and otherwise
// records nothing. With maxKeys set and that many keys in their window, a new
// key pushes out the one whose last hit is oldest: refusing it instead would
// let anybody with enough addresses shut the door for everyone else.
func (w *window) take(key string, n int, now time.Time) (bool, time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	kept := w.prune(key, now)
	if len(kept) == 0 && w.maxKeys > 0 && len(w.hits) >= w.maxKeys {
		w.evictStalest(now)
	}
	if used(kept)+n > w.max {
		if len(kept) == 0 {
			return false, w.span
		}
		return false, w.span - now.Sub(kept[0].at)
	}
	w.hits[key] = append(kept, hit{at: now, n: n})
	return true, 0
}

func (w *window) evictStalest(now time.Time) {
	var stalest string
	var last time.Time
	for k := range w.hits {
		hits := w.prune(k, now)
		if len(hits) > 0 && (last.IsZero() || hits[len(hits)-1].at.Before(last)) {
			stalest, last = k, hits[len(hits)-1].at
		}
	}
	if len(w.hits) >= w.maxKeys {
		delete(w.hits, stalest)
	}
}

// give takes back the hit of weight n that key made at at.
func (w *window) give(key string, n int, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	hits := w.hits[key]
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].at.Equal(at) && hits[i].n == n {
			w.hits[key] = append(hits[:i], hits[i+1:]...)
			break
		}
	}
	if len(w.hits[key]) == 0 {
		delete(w.hits, key)
	}
}

// left is how much key still has in its window.
func (w *window) left(key string, now time.Time) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.max - used(w.prune(key, now))
}

func used(hits []hit) int {
	n := 0
	for _, h := range hits {
		n += h.n
	}
	return n
}

// prune drops the hits that left key's window and returns the rest.
func (w *window) prune(key string, now time.Time) []hit {
	cutoff := now.Add(-w.span)
	hits := w.hits[key]
	kept := hits[:0]
	for _, h := range hits {
		if h.at.After(cutoff) {
			kept = append(kept, h)
		}
	}
	if len(kept) == 0 {
		delete(w.hits, key)
		return nil
	}
	w.hits[key] = kept
	return kept
}
