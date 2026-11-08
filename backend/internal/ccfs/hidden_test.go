package ccfs

import (
	"runtime"
	"testing"
)

func TestADotHidesANameExceptOnWindows(t *testing.T) {
	if got, want := HiddenByName(".git"), runtime.GOOS != "windows"; got != want {
		t.Errorf("HiddenByName(.git) = %v, want %v", got, want)
	}
	if HiddenByName("notes.txt") || HiddenByName("a.b") {
		t.Error("a name without a leading dot is hidden")
	}
}
