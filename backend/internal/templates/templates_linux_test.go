package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheTemplatesFolderComesFromUserDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if got := templatesDir(); got != filepath.Join(home, "Templates") {
		t.Errorf("without user-dirs.dirs: %q", got)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs := "# written by xdg-user-dirs-update\nXDG_DESKTOP_DIR=\"$HOME/Schreibtisch\"\nXDG_TEMPLATES_DIR=\"$HOME/Vorlagen\"\n"
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(dirs), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := templatesDir(); got != filepath.Join(home, "Vorlagen") {
		t.Errorf("from user-dirs.dirs: %q", got)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte("XDG_TEMPLATES_DIR=\"$HOME/\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := templatesDir(); got != "" {
		t.Errorf("the home folder as templates folder: %q", got)
	}
}
