package ccfs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestTheAttributeHidesAnEntryOnWindows(t *testing.T) {
	dir := t.TempDir()
	attrs := map[string]uint32{"desktop.ini": windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_SYSTEM, "Thumbs.db": windows.FILE_ATTRIBUTE_HIDDEN, ".git": 0, "boot.ini": windows.FILE_ATTRIBUTE_SYSTEM}
	for n, a := range attrs {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if a == 0 {
			continue
		}
		name, _ := windows.UTF16PtrFromString(p)
		if err := windows.SetFileAttributes(name, a); err != nil {
			t.Fatal(err)
		}
	}
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]bool{}
	for _, d := range list {
		got[d.Name()] = [2]bool{HiddenEntry(d), ProtectedEntry(d)}
	}
	want := map[string][2]bool{"desktop.ini": {true, true}, "Thumbs.db": {true, false}, ".git": {false, false}, "boot.ini": {false, false}}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("%s: hidden and protected = %v, want %v", n, got[n], w)
		}
	}
}
