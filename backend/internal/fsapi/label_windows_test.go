package fsapi

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsShowsTheLocalizedNameOfAFolder(t *testing.T) {
	f := newFixture(t)
	users := filepath.Join(f.share, "Users")
	for _, dir := range []string{users, filepath.Join(f.share, "plain")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		ini := "[.ShellClassInfo]\r\nLocalizedResourceName=@%SystemRoot%" + `\system32\shell32.dll,-21813` + "\r\n"
		if err := os.WriteFile(filepath.Join(dir, "desktop.ini"), []byte(ini), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	setAttrs(t, users, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_DIRECTORY)

	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user"))
	if got := byName(t, l.Items, "Users").Label; got == "" {
		t.Errorf("the folder has no name of its own: %q", got)
	}
	if got := byName(t, l.Items, "plain").Label; got != "" {
		t.Errorf("a folder without the attribute got %q", got)
	}
}
