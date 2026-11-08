package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// fakeXDG keeps the default for folders in a mimeapps.list of its own, as
// xdg-mime does in the person's configuration.
func fakeXDG(t *testing.T, before string) xdgHandler {
	t.Helper()
	dir := t.TempDir()
	h := xdgHandler{
		apps:     filepath.Join(dir, "share", "applications"),
		services: filepath.Join(dir, "share", "dbus-1", "services"),
		mimeapps: filepath.Join(dir, "config", "mimeapps.list"),
	}
	write := func(entry string) error {
		return writeFile(h.mimeapps, "[Default Applications]\ntext/plain=gedit.desktop;\n"+folderMime+"="+entry+";\n")
	}
	h.query = func() string {
		body, _ := os.ReadFile(h.mimeapps)
		for _, line := range strings.Split(string(body), "\n") {
			if v, ok := strings.CutPrefix(line, folderMime+"="); ok {
				return strings.Split(v, ";")[0]
			}
		}
		return ""
	}
	h.assign = write
	if before != "" {
		if err := write(before); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func TestFoldersOpenHereAndGoBackToTheFileManagerBefore(t *testing.T) {
	h := fakeXDG(t, "org.gnome.Nautilus.desktop")
	exe := "/opt/Cannon Commander/CannonCommander"
	for range 2 {
		if err := h.set(exe, desktop.Words{Open: "Ordner in CannonCommander öffnen"}); err != nil {
			t.Fatal(err)
		}
	}
	if !h.isSet(exe) || h.query() != entryName {
		t.Fatalf("not set, folders go to %s", h.query())
	}
	entry, _ := os.ReadFile(filepath.Join(h.apps, entryName))
	for _, want := range []string{`Exec="/opt/Cannon Commander/CannonCommander" %U`, "MimeType=inode/directory;", "NoDisplay=true", previousKey + "org.gnome.Nautilus.desktop"} {
		if !strings.Contains(string(entry), want+"\n") {
			t.Errorf("entry lacks %q:\n%s", want, entry)
		}
	}
	service, _ := os.ReadFile(filepath.Join(h.services, fileManager1+".service"))
	if !strings.Contains(string(service), "Name="+fileManager1+"\n") {
		t.Errorf("service file:\n%s", service)
	}

	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if h.isSet(exe) || h.query() != "org.gnome.Nautilus.desktop" {
		t.Fatalf("folders go to %s", h.query())
	}
	for _, f := range []string{filepath.Join(h.apps, entryName), filepath.Join(h.services, fileManager1+".service")} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s is still there", f)
		}
	}
}

func TestWithoutAnEarlierChoiceTheDesktopDecidesAgain(t *testing.T) {
	h := fakeXDG(t, "")
	if err := h.set("/usr/bin/cannoncommander", desktop.Words{}); err != nil {
		t.Fatal(err)
	}
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(h.mimeapps)
	if strings.Contains(string(body), folderMime) || !strings.Contains(string(body), "text/plain=gedit.desktop;") {
		t.Fatalf("mimeapps.list:\n%s", body)
	}
}
