package main

import (
	"io"
	"log"
	"maps"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

// An AppImage's program runs from a mount that is gone once it quits, so the
// restart, the folder entry, the modules and the update all name the AppImage.
func TestAnAppImageKeepsWhatOutlivesTheRunBesideItsFile(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("a Mac bundle carries its modules")
	}
	dir := t.TempDir()
	image := filepath.Join(dir, "CannonCommander.AppImage")
	t.Setenv("APPIMAGE", image)

	if exe, err := executable(); err != nil || exe != image {
		t.Errorf("the program starts as %q, %v", exe, err)
	}
	if folder, err := moduleFolder(); err != nil || folder != filepath.Join(dir, "modules") {
		t.Errorf("the modules live in %q, %v", folder, err)
	}
	up := newUpdater(log.New(io.Discard, "", 0))
	if up.u.Path != image || !maps.Equal(up.u.Assets, update.AppImages) {
		t.Errorf("the update replaces %q from %v", up.u.Path, up.u.Assets)
	}
}
