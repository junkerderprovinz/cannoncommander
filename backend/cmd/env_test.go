package cmd

import (
	"os"
	"slices"
	"testing"
)

func TestAdoptLegacyEnvCopiesUnsetNames(t *testing.T) {
	t.Setenv("FILEBROWSER_DEVMODE", "true")
	t.Setenv("FILEBROWSER_CONFIG", "/old/config.yaml")
	t.Setenv("CC_CONFIG", "/config/config.yaml")
	// t.Setenv restores the old value only for names it set itself.
	t.Setenv("CC_DEVMODE", "")
	os.Unsetenv("CC_DEVMODE")

	adopted := adoptLegacyEnv()

	if got := os.Getenv("CC_DEVMODE"); got != "true" {
		t.Errorf("CC_DEVMODE = %q, want true", got)
	}
	if got := os.Getenv("CC_CONFIG"); got != "/config/config.yaml" {
		t.Errorf("CC_CONFIG = %q, the CC_ value has to win", got)
	}
	if !slices.Contains(adopted, "FILEBROWSER_DEVMODE") {
		t.Errorf("adopted = %v, want FILEBROWSER_DEVMODE in it", adopted)
	}
	if slices.Contains(adopted, "FILEBROWSER_CONFIG") {
		t.Errorf("adopted = %v, FILEBROWSER_CONFIG was shadowed and must not be listed", adopted)
	}
}
