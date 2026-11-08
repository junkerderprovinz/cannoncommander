package archives

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindTakesThePathAndThenTheInstalledPrograms(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "7z")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := installed
	t.Cleanup(func() { installed = old })
	installed = func(n string) string {
		if n == "7z" {
			return exe
		}
		return ""
	}
	t.Setenv("PATH", t.TempDir())
	if got := Find("7zz", "7z"); got != exe {
		t.Errorf("Find = %q, want the installed %q", got, exe)
	}
	if got := Find("rar"); got != "" {
		t.Errorf("Find(rar) = %q without any rar", got)
	}
	t.Setenv("PATH", dir)
	if got := Find("7zz", "7z"); got == "" {
		t.Error("the program on the PATH is not found")
	}
}
