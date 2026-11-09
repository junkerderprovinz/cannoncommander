package main

import (
	"os"
	"os/exec"
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
	h.reload = func() {}
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

func TestExecQuotesAProgramOnlyWhenItMust(t *testing.T) {
	for exe, want := range map[string]string{
		"/opt/CannonCommander/CannonCommander":  "/opt/CannonCommander/CannonCommander",
		"/opt/Cannon Commander/CannonCommander": `"/opt/Cannon Commander/CannonCommander"`,
		`/home/jdp/Apps/cc$1/CannonCommander`:   `"/home/jdp/Apps/cc\\$1/CannonCommander"`,
		"/home/jdp/100%/CannonCommander":        "/home/jdp/100%%/CannonCommander",
		`/home/jdp/a"b/CannonCommander`:         `"/home/jdp/a\\"b/CannonCommander"`,
	} {
		if got := quoteExec(exe); got != want {
			t.Errorf("%s: got %s, want %s", exe, got, want)
		}
	}
}

func TestTheServiceFileQuotesAsAShellDoes(t *testing.T) {
	for exe, want := range map[string]string{
		"/opt/CannonCommander/CannonCommander":  "/opt/CannonCommander/CannonCommander",
		"/opt/Cannon Commander/CannonCommander": "'/opt/Cannon Commander/CannonCommander'",
		"/home/jdp/it's/CannonCommander":        `'/home/jdp/it'\''s/CannonCommander'`,
	} {
		if got := quoteService(exe); got != want {
			t.Errorf("%s: got %s, want %s", exe, got, want)
		}
	}
}

// listsIn lays out a desktop's configuration and data folders, writes the
// given mimeapps.list files and installs the entries.
func listsIn(t *testing.T, lists map[string]string, entries ...string) (xdgDirs, string) {
	t.Helper()
	base := t.TempDir()
	d := xdgDirs{
		config:   []string{filepath.Join(base, "config"), filepath.Join(base, "etc", "xdg")},
		data:     []string{filepath.Join(base, "share"), filepath.Join(base, "usr", "share")},
		desktops: []string{"ubuntu", "gnome"},
	}
	for name, body := range lists {
		if err := writeFile(filepath.Join(base, name), body); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range entries {
		if err := writeFile(filepath.Join(base, "usr", "share", "applications", e), "[Desktop Entry]\n"); err != nil {
			t.Fatal(err)
		}
	}
	return d, base
}

func TestTheDefaultForFoldersIsReadAsGLibReadsIt(t *testing.T) {
	d, _ := listsIn(t, map[string]string{
		"config/mimeapps.list":                       "[Added Associations]\ninode/directory=gone.desktop;\n\n[Default Applications]\ninode/directory=gone.desktop;" + entryName + ";\n",
		"usr/share/applications/gnome-mimeapps.list": "[Default Applications]\ninode/directory=org.gnome.Nautilus.desktop\n",
		"etc/xdg/ubuntu-mimeapps.list":               "[Default Applications]\ntext/plain=org.gnome.TextEditor.desktop\n",
	}, entryName, "org.gnome.Nautilus.desktop")
	if got := defaultFor(folderMime, d); got != entryName {
		t.Fatalf("got %q", got)
	}
}

func TestADesktopsOwnListComesFirst(t *testing.T) {
	d, _ := listsIn(t, map[string]string{
		"config/mimeapps.list":       "[Default Applications]\ninode/directory=" + entryName + "\n",
		"config/gnome-mimeapps.list": "[Default Applications]\ninode/directory=org.gnome.Nautilus.desktop\n",
	}, entryName, "org.gnome.Nautilus.desktop")
	if got := defaultFor(folderMime, d); got != "org.gnome.Nautilus.desktop" {
		t.Fatalf("got %q", got)
	}
}

func TestWithoutAnyListThereIsNoDefault(t *testing.T) {
	d, _ := listsIn(t, nil)
	if got := defaultFor(folderMime, d); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTheBusLearnsOfTheServiceFile(t *testing.T) {
	h := fakeXDG(t, "")
	reloads := 0
	h.reload = func() { reloads++ }
	if err := h.set("/usr/bin/cannoncommander", desktop.Words{}); err != nil {
		t.Fatal(err)
	}
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if reloads != 2 {
		t.Fatalf("%d reloads", reloads)
	}
}

// The real xdg-utils and gio decide what a desktop sees. Ubuntu 24.04 ships
// xdg-mime 1.1.3, which looks up the first word of Exec as it stands and,
// when that fails, falls back to the desktop's own file manager.
func TestXdgMimeAndGioSeeTheSwitch(t *testing.T) {
	for _, p := range []string{"xdg-mime", "gio"} {
		if _, err := exec.LookPath(p); err != nil {
			t.Skip(p + " is not installed")
		}
	}
	home := t.TempDir()
	system := filepath.Join(t.TempDir(), "usr", "share")
	nautilus := "[Desktop Entry]\nType=Application\nName=Files\nExec=/bin/sh %U\nMimeType=inode/directory;\n"
	if err := writeFile(filepath.Join(system, "applications", "org.gnome.Nautilus.desktop"), nautilus); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(system, "applications", "mimeinfo.cache"), "[MIME Cache]\ninode/directory=org.gnome.Nautilus.desktop;\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_DATA_DIRS", system)
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	exe := filepath.Join(home, "Apps", "CannonCommander")
	if err := writeFile(exe, "#!/bin/sh\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(exe, 0o755); err != nil {
		t.Fatal(err)
	}
	h := newFolderHandler().(xdgHandler)
	h.reload = func() {}
	if err := h.set(exe, desktop.Words{}); err != nil {
		t.Fatal(err)
	}
	if !h.isSet(exe) {
		t.Error("the switch reads off")
	}
	out, _ := exec.Command("xdg-mime", "query", "default", folderMime).Output()
	if got := strings.TrimSpace(string(out)); got != entryName {
		t.Errorf("xdg-mime says %q", got)
	}
	cmd := exec.Command("gio", "mime", folderMime)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	out, _ = cmd.Output()
	if first, _, _ := strings.Cut(string(out), "\n"); !strings.HasPrefix(first, "Default application") || !strings.HasSuffix(first, ": "+entryName) {
		t.Errorf("gio says %s", out)
	}
	if err := h.unset(); err != nil {
		t.Fatal(err)
	}
	if h.isSet(exe) {
		t.Error("the switch reads on")
	}
}
