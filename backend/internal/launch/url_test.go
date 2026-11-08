package launch

import (
	"os"
	"path/filepath"
	"testing"
)

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
