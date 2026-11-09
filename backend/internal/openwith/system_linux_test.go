//go:build desktop

package openwith

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// fakeGio is a gio that starts a program which, like most, keeps the output
// it was handed open for as long as it runs.
func fakeGio(t *testing.T, exit int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gio")
	script := "#!/bin/sh\nsleep 20 &\necho launched\nexit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAProgramGioStartsDoesNotHoldTheRequest(t *testing.T) {
	start := time.Now()
	if err := gioLaunch(fakeGio(t, 0), "/usr/share/applications/editor.desktop", "/home/jdp/notes.txt"); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("took %s", took)
	}
}

func TestAFailedLaunchIsReported(t *testing.T) {
	if err := gioLaunch(fakeGio(t, 1), "/usr/share/applications/editor.desktop", "/home/jdp/notes.txt"); err == nil {
		t.Fatal("no error")
	}
}
