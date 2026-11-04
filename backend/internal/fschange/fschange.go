// Package fschange tells other packages about the changes CannonCommander
// makes on disk itself, so labels and the index can follow a rename or a
// move right away instead of waiting for the watcher. All paths are API
// paths.
package fschange

import "sync"

// Observer gets told about changes. Any of its functions may be nil. They
// run on the goroutine that made the change and must return quickly.
type Observer struct {
	Created func(path string)
	Moved   func(from, to string)
	Removed func(path string)
}

var (
	mu        sync.RWMutex
	nextID    int
	observers = map[int]Observer{}
)

// Observe adds o and returns a function that removes it again.
func Observe(o Observer) (stop func()) {
	mu.Lock()
	defer mu.Unlock()
	nextID++
	id := nextID
	observers[id] = o
	return func() {
		mu.Lock()
		defer mu.Unlock()
		delete(observers, id)
	}
}

func each(f func(Observer)) {
	mu.RLock()
	list := make([]Observer, 0, len(observers))
	for _, o := range observers {
		list = append(list, o)
	}
	mu.RUnlock()
	for _, o := range list {
		f(o)
	}
}

// Created reports a new file or folder.
func Created(path string) {
	each(func(o Observer) {
		if o.Created != nil {
			o.Created(path)
		}
	})
}

// Moved reports that the entry at from lives at to, after a rename or a move. For a
// folder it covers everything inside.
func Moved(from, to string) {
	each(func(o Observer) {
		if o.Moved != nil {
			o.Moved(from, to)
		}
	})
}

// Removed reports that path and everything below it is gone for good.
func Removed(path string) {
	each(func(o Observer) {
		if o.Removed != nil {
			o.Removed(path)
		}
	})
}
