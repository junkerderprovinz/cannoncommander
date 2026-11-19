package appimage

import (
	"os"
	"testing"
)

func TestTheRuntimeNamesTheAppImage(t *testing.T) {
	t.Setenv("APPIMAGE", "/home/jane/Apps/CannonCommander.AppImage")
	if got := Path(); got != "/home/jane/Apps/CannonCommander.AppImage" {
		t.Errorf("Path() = %q inside an AppImage", got)
	}
}

func TestOutsideAnAppImageThereIsNoPath(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	if got := Path(); got != "" {
		t.Errorf("Path() = %q outside an AppImage", got)
	}
}

// What starts the program again after it quits has to start the AppImage,
// since the program inside it lives on a mount that goes with the run.
func TestAnAppImageStartsFromItsFile(t *testing.T) {
	t.Setenv("APPIMAGE", "/home/jane doe/Apps/CannonCommander.AppImage")
	if got, err := Executable(); err != nil || got != "/home/jane doe/Apps/CannonCommander.AppImage" {
		t.Errorf("Executable() = %q, %v inside an AppImage", got, err)
	}
}

func TestOutsideAnAppImageTheProgramStartsItself(t *testing.T) {
	t.Setenv("APPIMAGE", "")
	want, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Executable(); err != nil || got != want {
		t.Errorf("Executable() = %q, %v, want %q", got, err, want)
	}
}
