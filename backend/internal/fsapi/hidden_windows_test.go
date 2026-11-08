package fsapi

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func setAttrs(t *testing.T, p string, attrs uint32) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, attrs); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsHidesByTheAttributeAndShowsDotFiles(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Join(f.share, "fotos")
	for _, n := range []string{"desktop.ini", "Thumbs.db", ".gitignore"} {
		write(t, filepath.Join(dir, n), 1)
	}
	setAttrs(t, filepath.Join(dir, "desktop.ini"), windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM)
	setAttrs(t, filepath.Join(dir, "Thumbs.db"), windows.FILE_ATTRIBUTE_HIDDEN)

	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos"))
	if ini := byName(t, l.Items, "desktop.ini"); !ini.Hidden || !ini.System {
		t.Errorf("desktop.ini = %+v", ini)
	}
	if thumbs := byName(t, l.Items, "Thumbs.db"); !thumbs.Hidden || thumbs.System {
		t.Errorf("Thumbs.db = %+v", thumbs)
	}
	if dot := byName(t, l.Items, ".gitignore"); dot.Hidden || dot.System {
		t.Errorf(".gitignore = %+v", dot)
	}
}
