package tools

import (
	"net/http"
	"testing"

	"golang.org/x/sys/windows"
)

func attrsOf(t *testing.T, p string) uint32 {
	t.Helper()
	name, _ := windows.UTF16PtrFromString(p)
	a, err := windows.GetFileAttributes(name)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestWindowsAttributesGoOnAndOff(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "a")
	real := e.real("/mnt/user/fotos/a.txt")
	set := func(flags map[string]bool) {
		t.Helper()
		if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/a.txt"}, Flags: flags}, nil); code != http.StatusOK {
			t.Fatalf("attrs %v: %d", flags, code)
		}
	}

	set(map[string]bool{"readOnly": true, "hidden": true, "archive": false})
	if a := attrsOf(t, real); a&windows.FILE_ATTRIBUTE_READONLY == 0 || a&windows.FILE_ATTRIBUTE_HIDDEN == 0 || a&windows.FILE_ATTRIBUTE_ARCHIVE != 0 {
		t.Errorf("attributes %#x", a)
	}
	set(map[string]bool{"readOnly": false, "hidden": false})
	if a := attrsOf(t, real); a&(windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_HIDDEN) != 0 {
		t.Errorf("attributes %#x", a)
	}
	set(map[string]bool{"pinned": true})
	if a := attrsOf(t, real); a&attrPinned == 0 || a&attrUnpinned != 0 {
		t.Errorf("kept on the device %#x", a)
	}
	set(map[string]bool{"pinned": false})
	if a := attrsOf(t, real); a&attrPinned != 0 || a&attrUnpinned == 0 {
		t.Errorf("space freed %#x", a)
	}
}
