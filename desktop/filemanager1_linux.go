package main

import (
	"log"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const fileManager1Path = dbus.ObjectPath("/org/freedesktop/FileManager1")

const fileManager1XML = `<node>
  <interface name="org.freedesktop.FileManager1">
    <method name="ShowFolders"><arg type="as" name="URIs" direction="in"/><arg type="s" name="StartupId" direction="in"/></method>
    <method name="ShowItems"><arg type="as" name="URIs" direction="in"/><arg type="s" name="StartupId" direction="in"/></method>
    <method name="ShowItemProperties"><arg type="as" name="URIs" direction="in"/><arg type="s" name="StartupId" direction="in"/></method>
  </interface>` + introspect.IntrospectDeclarationString + `</node>`

// folderService answers org.freedesktop.FileManager1, which browsers, mail
// programs and the desktop call to show a folder or a file in its folder.
type folderService struct {
	show func([]opening)
}

func (f folderService) ShowFolders(uris []string, _ string) *dbus.Error {
	open, _ := handedOver(uris, "/")
	f.show(open)
	return nil
}

// ShowItems opens the folder each item lies in with the cursor on it, also
// for an item that is a folder itself.
func (f folderService) ShowItems(uris []string, _ string) *dbus.Error {
	var open []opening
	for _, u := range uris {
		p := u
		if parsed, err := url.Parse(u); err == nil && parsed.Scheme == "file" {
			p = parsed.Path
		}
		if filepath.IsAbs(p) {
			open = append(open, opening{Folder: filepath.Dir(p), Select: filepath.Base(p)})
		}
	}
	f.show(open)
	return nil
}

func (f folderService) ShowItemProperties(uris []string, startup string) *dbus.Error {
	return f.ShowItems(uris, startup)
}

// serveFolders puts the service on conn and asks for its name. A file manager
// that holds the name and lets it go hands it over; one that keeps it keeps it
// until it ends, and the app gets it then.
func serveFolders(conn *dbus.Conn, show func([]opening)) error {
	if err := conn.Export(folderService{show}, fileManager1Path, fileManager1); err != nil {
		return err
	}
	if err := conn.Export(introspect.Introspectable(fileManager1XML), fileManager1Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return err
	}
	_, err := conn.RequestName(fileManager1, dbus.NameFlagReplaceExisting|dbus.NameFlagAllowReplacement)
	return err
}

func stopServing(conn *dbus.Conn) {
	_, _ = conn.ReleaseName(fileManager1)
	_ = conn.Export(nil, fileManager1Path, fileManager1)
	_ = conn.Export(nil, fileManager1Path, "org.freedesktop.DBus.Introspectable")
}

var sessionBus struct {
	sync.Mutex
	conn *dbus.Conn
}

// offerFolders serves FileManager1 on the session bus while the app is the
// file manager.
func offerFolders(s *shell, on bool) {
	sessionBus.Lock()
	defer sessionBus.Unlock()
	if !on {
		if sessionBus.conn != nil {
			stopServing(sessionBus.conn)
		}
		return
	}
	if sessionBus.conn == nil {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			log.Printf("no session bus to show folders for other programs: %v", err)
			return
		}
		sessionBus.conn = conn
	}
	if err := serveFolders(sessionBus.conn, s.show); err != nil {
		log.Printf("could not offer %s: %v", fileManager1, err)
	}
}
