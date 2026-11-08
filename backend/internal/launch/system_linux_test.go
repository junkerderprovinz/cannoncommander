package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAFileStartsSomethingWhenItIsAnExecutableProgram(t *testing.T) {
	dir := t.TempDir()
	files := []struct {
		name string
		body string
		mode os.FileMode
		want bool
	}{
		{"tool", "\x7fELF\x02\x01", 0o755, true},
		{"build.sh", "#!/bin/sh\necho hi\n", 0o755, true},
		{"notes.sh", "#!/bin/sh\necho hi\n", 0o644, false},
		// A whole stick of FAT marks every file executable.
		{"photo.jpg", "\xff\xd8\xff\xe0", 0o777, false},
		{"firefox.desktop", "[Desktop Entry]\n", 0o644, true},
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, []byte(f.body), f.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, f.mode); err != nil {
			t.Fatal(err)
		}
		if got := startsSomething(p); got != f.want {
			t.Errorf("%s: got %v", f.name, got)
		}
	}
}

func TestAnInternetShortcutOpensItsAddressInTheBrowser(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	web := write("Wetter.url", "[InternetShortcut]\r\nURL=https://wetter.example/wien\r\n")
	local := write("Datei.url", "[InternetShortcut]\r\nURL=file:///etc/passwd\r\n")
	notes := write("notes.txt", "x")
	for file, want := range map[string]string{web: "https://wetter.example/wien", local: local, notes: notes} {
		if got := toOpen(file); got != want {
			t.Errorf("toOpen(%s) = %q, want %q", filepath.Base(file), got, want)
		}
	}
}
