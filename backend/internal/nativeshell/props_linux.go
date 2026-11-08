package nativeshell

import (
	"context"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

const fileManager = "org.freedesktop.FileManager1"

var session struct {
	sync.Mutex
	conn    *dbus.Conn
	has     bool
	checked time.Time
}

// HasProperties reports whether another program offers the file manager
// interface on the session bus, which shows a properties dialog. The answer
// is kept for a few seconds, as the app asks with every menu.
func HasProperties() bool {
	session.Lock()
	defer session.Unlock()
	if time.Since(session.checked) > 5*time.Second {
		session.has = offered()
		session.checked = time.Now()
	}
	return session.has
}

// Properties shows the file manager's properties dialog of files.
func Properties(files []string) error {
	session.Lock()
	ok := offered()
	session.has, session.checked = ok, time.Now()
	conn := session.conn
	session.Unlock()
	if !ok {
		return ErrUnsupported
	}
	uris := make([]string, len(files))
	for i, f := range files {
		uris[i] = xdg.FileURI(f)
	}
	// The bus may have to start the file manager first.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return conn.Object(fileManager, "/org/freedesktop/FileManager1").CallWithContext(ctx, fileManager+".ShowItemProperties", 0, uris, "").Err
}

// offered reports whether a program other than this one owns the file
// manager's name, or the bus can start one that does. The caller holds the
// session lock.
func offered() bool {
	if session.conn == nil || !session.conn.Connected() {
		conn, err := connect()
		if err != nil {
			return false
		}
		session.conn = conn
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bus := session.conn.BusObject()
	var owner string
	if bus.CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, fileManager).Store(&owner) == nil {
		var pid uint32
		err := bus.CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, owner).Store(&pid)
		return err != nil || int(pid) != os.Getpid()
	}
	var names []string
	return bus.CallWithContext(ctx, "org.freedesktop.DBus.ListActivatableNames", 0).Store(&names) == nil && slices.Contains(names, fileManager)
}

// connect opens a connection of this package's own to the session bus. It
// never starts a bus where there is none.
func connect() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, err
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// CanRunAsAdmin reports false, as the app does not ask for a password
// through polkit.
func CanRunAsAdmin(string) bool { return false }

// RunAsAdmin returns ErrUnsupported, as the app does not ask for a password
// through polkit.
func RunAsAdmin(string) error { return ErrUnsupported }
