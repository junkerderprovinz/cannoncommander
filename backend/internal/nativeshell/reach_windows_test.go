package nativeshell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A server that answers nothing, in an address range kept for documentation.
const nowhere = `\\192.0.2.1\share\evil.ico`

func TestAShortcutWhoseIconLiesOnAServerShowsTheTypeIcon(t *testing.T) {
	lnk := filepath.Join(t.TempDir(), "Bericht.lnk")
	target := filepath.Join(os.Getenv("SystemRoot"), "notepad.exe")
	script := "$l = (New-Object -ComObject WScript.Shell).CreateShortcut('" + lnk + "'); $l.TargetPath = '" + target + "'; $l.IconLocation = '" + nowhere + ",0'; $l.Save()"
	if out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	assertTypeIcon(t, lnk, "Bericht.lnk", false)
}

func TestAWebLinkWhoseIconLiesOnAServerShowsTheTypeIcon(t *testing.T) {
	url := filepath.Join(t.TempDir(), "Seite.url")
	if err := os.WriteFile(url, []byte("[InternetShortcut]\r\nURL=https://example.org/\r\nIconFile="+nowhere+"\r\nIconIndex=0\r\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	assertTypeIcon(t, url, "Seite.url", false)
}

func TestAFolderWhoseDesktopIniNamesAServerShowsTheFolderIcon(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Projekte")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ini := "[.ShellClassInfo]\r\nIconResource=" + nowhere + ",0\r\n"
	if err := os.WriteFile(filepath.Join(dir, "desktop.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(dir)
	if err := windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_DIRECTORY); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_DIRECTORY) })
	assertTypeIcon(t, dir, "folder", true)
}

func assertTypeIcon(t *testing.T, file, name string, dir bool) {
	t.Helper()
	start := time.Now()
	got, _, err := Icon(file, 32)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("the lookup took %s, as if it asked the server", d)
	}
	want, _, err := TypeIcon(name, dir, 32)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("the icon is not the one of the type")
	}
}

func shortcutWith(t *testing.T, dir, name, target, icon string) string {
	t.Helper()
	lnk := filepath.Join(dir, name)
	script := "$l = (New-Object -ComObject WScript.Shell).CreateShortcut('" + lnk + "'); $l.TargetPath = '" + target + "'"
	if icon != "" {
		script += "; $l.IconLocation = '" + icon + ",0'"
	}
	if out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script+"; $l.Save()").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return lnk
}

func TestOnlyIconsKeptOnThisComputerAreLookedUp(t *testing.T) {
	dir := t.TempDir()
	notepad := filepath.Join(os.Getenv("SystemRoot"), "notepad.exe")
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o666); err != nil {
			t.Fatal(err)
		}
		return p
	}
	folder := func(name, ini string, readOnly bool) string {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "desktop.ini"), []byte(ini), 0o644); err != nil {
			t.Fatal(err)
		}
		if readOnly {
			u, _ := windows.UTF16PtrFromString(p)
			_ = windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_DIRECTORY)
			t.Cleanup(func() { _ = windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_DIRECTORY) })
		}
		return p
	}
	for _, c := range []struct {
		name string
		file string
		away bool
	}{
		{"shortcut with an icon on a server", shortcutWith(t, dir, "a.lnk", notepad, nowhere), true},
		{"shortcut with a local icon", shortcutWith(t, dir, "b.lnk", notepad, notepad), false},
		{"shortcut to a program on a server", shortcutWith(t, dir, "c.lnk", `\\192.0.2.1\share\tool.exe`, ""), true},
		{"shortcut to a local program", shortcutWith(t, dir, "d.lnk", notepad, ""), false},
		{"web link with an icon on a server", write("a.url", "[InternetShortcut]\r\nURL=https://example.org/\r\nIconFile="+nowhere+"\r\n"), true},
		{"web link with an icon on the web", write("b.url", "[InternetShortcut]\r\nURL=https://example.org/\r\nIconFile=https://example.org/x.ico\r\n"), true},
		{"plain web link", write("c.url", "[InternetShortcut]\r\nURL=https://example.org/\r\n"), false},
		{"read-only folder naming a server", folder("away", "[.ShellClassInfo]\r\nIconResource="+nowhere+",0\r\n", true), true},
		{"read-only folder naming a file on a server", folder("file", "[.ShellClassInfo]\r\nIconFile="+nowhere+"\r\nIconIndex=0\r\n", true), true},
		{"plain folder naming a server", folder("plain", "[.ShellClassInfo]\r\nIconResource="+nowhere+",0\r\n", false), false},
		{"read-only folder with a local icon", folder("local", "[.ShellClassInfo]\r\nIconResource=%SystemRoot%\\system32\\imageres.dll,-3\r\n", true), false},
		{"plain file", write("notes.txt", "hi"), false},
	} {
		if got := reachesOut(c.file); got != c.away {
			t.Errorf("%s: reaches out %v, want %v", c.name, got, c.away)
		}
	}
}
