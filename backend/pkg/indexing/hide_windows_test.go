package indexing

import (
	"testing"

	"golang.org/x/sys/windows"
)

// hide gives p the hidden attribute, which is what hides it on Windows.
func hide(t *testing.T, p string, system bool) {
	t.Helper()
	attrs := uint32(windows.FILE_ATTRIBUTE_HIDDEN)
	if system {
		attrs |= windows.FILE_ATTRIBUTE_SYSTEM
	}
	setAttrs(t, p, attrs)
}

func setAttrs(t *testing.T, p string, attrs uint32) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	old, err := windows.GetFileAttributes(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, old|attrs); err != nil {
		t.Fatal(err)
	}
}
