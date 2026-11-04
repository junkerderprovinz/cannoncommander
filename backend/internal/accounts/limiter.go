package accounts

import (
	"sync"
	"time"
)

// limiter counts failed attempts per key and blocks a key that failed too
// often within the window, so passwords and codes cannot be guessed.
type limiter struct {
	max    int
	window time.Duration

	mu    sync.Mutex
	fails map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *limiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) >= l.max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.recent(key), time.Now())
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// recent drops the failures that left the window. Call with l.mu held.
func (l *limiter) recent(key string) []time.Time {
	cut := time.Now().Add(-l.window)
	list := l.fails[key]
	i := 0
	for i < len(list) && list[i].Before(cut) {
		i++
	}
	list = list[i:]
	if len(list) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = list
	return list
}

// wait returns how long key stays blocked.
func (l *limiter) wait(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.recent(key)
	if len(list) < l.max {
		return 0
	}
	return time.Until(list[len(list)-l.max].Add(l.window))
}
