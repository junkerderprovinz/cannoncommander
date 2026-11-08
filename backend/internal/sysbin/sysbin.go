// Package sysbin reads and fills the trash of the operating system: the
// Recycle Bin on Windows and the freedesktop.org trash on Linux. The desktop
// app puts deleted files there, so the Explorer, Nautilus and CannonCommander
// all see and restore the same entries.
package sysbin

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNoBin says that the drive or volume of a path has no trash, as a
	// network drive or a USB stick on Windows.
	ErrNoBin = errors.New("this drive has no trash")
	// ErrNotFound is an id that names no entry in the trash.
	ErrNotFound = errors.New("no such entry in the trash")
	// ErrUnsupported comes from Open on a system without a known trash.
	ErrUnsupported = errors.New("the trash of this system is not supported")
	// ErrCancelled is a delete the person stopped in a dialog of the system.
	ErrCancelled = errors.New("cancelled")
)

// Entry is one item in the trash.
type Entry struct {
	// ID names the entry for Get, Restore and Forget. It stays the same as
	// long as the entry is there.
	ID string
	// Name is the name the item had, Origin the absolute path it was deleted
	// from.
	Name   string
	Origin string
	// Path is where the item lies inside the trash.
	Path    string
	Dir     bool
	Size    int64
	Deleted time.Time
}

// Bin is the trash of the system.
type Bin interface {
	// Put moves the file or folder at path into the trash.
	Put(path string) (Entry, error)
	// Adopt moves path into the trash as if it had been deleted from origin
	// at the given time, for entries that another trash held before.
	Adopt(path, origin string, deleted time.Time) (Entry, error)
	List() ([]Entry, error)
	Get(id string) (Entry, error)
	// Restore moves an entry to the path to, which must not exist yet.
	Restore(id, to string) error
	// Forget removes what the trash knows about an entry once the caller has
	// deleted its Path.
	Forget(id string) error
}

// makeID joins the place of a trash and the name of an entry in it. The
// name goes in hex, so the id needs no escaping in a URL.
func makeID(place, name string) string {
	return place + "." + hex.EncodeToString([]byte(name))
}

// splitID is the reverse of makeID. It refuses a name that could lead out of
// its folder.
func splitID(id string) (place, name string, err error) {
	place, enc, ok := strings.Cut(id, ".")
	if !ok || place == "" {
		return "", "", ErrNotFound
	}
	raw, err := hex.DecodeString(enc)
	name = string(raw)
	if err != nil || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return "", "", ErrNotFound
	}
	return place, name, nil
}
