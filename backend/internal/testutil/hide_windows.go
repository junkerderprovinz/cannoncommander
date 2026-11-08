package testutil

import (
	"testing"

	"golang.org/x/sys/windows"
)

// Hide hides p the way the system does. A test tree names hidden entries
// with a dot, which hides them everywhere but on Windows, where only the
// attribute counts.
func Hide(t testing.TB, p string) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := windows.GetFileAttributes(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, attrs|windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
}
