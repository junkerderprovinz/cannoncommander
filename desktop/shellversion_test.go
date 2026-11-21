package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

func TestTheShellHearsOfEachVersionOnce(t *testing.T) {
	var told []string
	was := update.ShellChanged
	update.ShellChanged = func(path string) { told = append(told, path) }
	t.Cleanup(func() { update.ShellChanged = was })
	mark := filepath.Join(t.TempDir(), "shell-version")

	shellKnows(mark)
	shellKnows(mark)
	if len(told) != 1 {
		t.Fatalf("told the shell %d times", len(told))
	}
	if err := os.WriteFile(mark, []byte("v0.9.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	shellKnows(mark)
	if len(told) != 2 {
		t.Errorf("another version told the shell %d times in all", len(told))
	}
}
