package indexing

import (
	"container/list"
	"sync"
	"time"
)

// realPaths remembers resolved real paths for two days.
var realPaths = newRealPathCache(realPathLimit, 48*time.Hour)

// realPathLimit bounds the real paths kept at once. A lookup stores the
// path and every folder above it, so a person browsing for two days stays
// well inside it, while a client walking a whole share, which adds an entry
// per folder, only pushes out the oldest. An entry takes about 350 bytes,
// so a full cache holds about 3.5 MB, and a miss costs one symlink check.
const realPathLimit = 10000

// realPathCache keeps up to limit real paths and drops the one used least
// recently when a new one would go over.
type realPathCache struct {
	mu    sync.Mutex
	limit int
	ttl   time.Duration
	byKey map[string]*list.Element
	// order holds *realPath values, the most recently used first.
	order *list.List
}

type realPath struct {
	key   string
	path  string
	isDir bool
	until time.Time
}

func newRealPathCache(limit int, ttl time.Duration) *realPathCache {
	return &realPathCache{limit: limit, ttl: ttl, byKey: map[string]*list.Element{}, order: list.New()}
}

func (c *realPathCache) get(key string) (path string, isDir, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byKey[key]
	if !ok {
		return "", false, false
	}
	rp := e.Value.(*realPath)
	if time.Now().After(rp.until) {
		c.order.Remove(e)
		delete(c.byKey, key)
		return "", false, false
	}
	c.order.MoveToFront(e)
	return rp.path, rp.isDir, true
}

func (c *realPathCache) set(key, path string, isDir bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	until := time.Now().Add(c.ttl)
	if e, ok := c.byKey[key]; ok {
		rp := e.Value.(*realPath)
		rp.path, rp.isDir, rp.until = path, isDir, until
		c.order.MoveToFront(e)
		return
	}
	c.byKey[key] = c.order.PushFront(&realPath{key: key, path: path, isDir: isDir, until: until})
	for c.order.Len() > c.limit {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.byKey, oldest.Value.(*realPath).key)
	}
}

func (c *realPathCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
