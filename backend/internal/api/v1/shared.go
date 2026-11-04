package v1

import (
	"database/sql"
	"sync"
)

type sharedKey struct {
	db   *sql.DB
	name string
}

type sharedEntry struct {
	once sync.Once
	val  any
	err  error
}

var (
	sharedMu sync.Mutex
	shared   = map[sharedKey]*sharedEntry{}
)

// Shared returns the service called name for the database in d and builds
// it on first use. A job queue or a trash has to exist once per database,
// while several packages bind routes to it and hook into it. build may ask
// for other shared services, but not for its own.
func Shared[T any](d Deps, name string, build func() (T, error)) (T, error) {
	key := sharedKey{d.DB, name}
	sharedMu.Lock()
	e, ok := shared[key]
	if !ok {
		e = &sharedEntry{}
		shared[key] = e
	}
	sharedMu.Unlock()
	e.once.Do(func() { e.val, e.err = build() })
	if e.err != nil {
		var zero T
		Forget(d, name)
		return zero, e.err
	}
	return e.val.(T), nil
}

// Forget drops the shared service called name for the database in d, so
// the next Shared call builds a fresh one.
func Forget(d Deps, name string) {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	delete(shared, sharedKey{d.DB, name})
}
