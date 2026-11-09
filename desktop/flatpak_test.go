package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const flatpakID = "io.github.junkerderprovinz.cannoncommander"

// The sandbox lets the app own bus names under its own ID alone, and the
// desktop finds the app's launcher for a window by the same name.
func TestAFlatpakGoesByItsID(t *testing.T) {
	t.Setenv("FLATPAK_ID", flatpakID)
	t.Setenv("CC_DATA", "")
	opts := linuxOptions()
	if opts.ApplicationID != flatpakID || opts.ProgramName != flatpakID {
		t.Errorf("application ID %q, program name %q, want both %q", opts.ApplicationID, opts.ProgramName, flatpakID)
	}
	if got := instanceID("/home/jane/.var/app/" + flatpakID + "/config/CannonCommander"); got != flatpakID {
		t.Errorf("single-instance lock %q, want %q", got, flatpakID)
	}
	t.Setenv("CC_DATA", "/srv/cc")
	if got := instanceID("/srv/cc"); !strings.HasPrefix(got, flatpakID+".") {
		t.Errorf("a copy on its own data folder locks %q, outside the app's names", got)
	}
}

func TestOutsideAFlatpakTheNamesStay(t *testing.T) {
	t.Setenv("FLATPAK_ID", "")
	t.Setenv("CC_DATA", "")
	opts := linuxOptions()
	if opts.ApplicationID != "" || opts.ProgramName != "CannonCommander" {
		t.Errorf("application ID %q, program name %q", opts.ApplicationID, opts.ProgramName)
	}
	if got := instanceID("x"); got != appID {
		t.Errorf("single-instance lock %q", got)
	}
}

func TestAFlatpakNeverUpdatesItself(t *testing.T) {
	t.Setenv("FLATPAK_ID", flatpakID)
	s := &shell{}
	if s.AutoUpdate() {
		t.Error("the switch reads on in a Flatpak")
	}
	if err := s.SetAutoUpdate(true); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("turning it on: %v", err)
	}

	var out bytes.Buffer
	up := newUpdater(log.New(&out, "", 0))
	done := make(chan struct{})
	go func() {
		up.run(context.Background(), func() bool { return true })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a Flatpak keeps waiting to update itself")
	}
	if !strings.Contains(out.String(), "software centre") {
		t.Errorf("logged %q", out.String())
	}
}

// The switch would write an entry and a service file the computer never
// reads, so a Flatpak hides it. Its own desktop entry names it for folders
// instead, which the system settings offer as the default, and with the
// switch goes the tray it stands by in.
func TestAFlatpakLeavesTheFileManagerToTheSystemSettings(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Flatpak is Linux only")
	}
	t.Setenv("FLATPAK_ID", flatpakID)
	if h := newFolderHandler(); h != nil {
		t.Fatalf("got %T", h)
	}
	s := &shell{}
	s.folders = newFolderHandler()
	if _, err := s.IsFileManager(); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("the switch shows: %v", err)
	}
	entry, err := os.ReadFile(filepath.Join("..", "flatpak", flatpakID+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(entry, []byte("MimeType=inode/directory;")) || !bytes.Contains(entry, []byte("Exec=cannoncommander %U")) {
		t.Errorf("the desktop entry does not open folders:\n%s", entry)
	}
}
