package templates

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func use(t *testing.T, list []Template) {
	t.Helper()
	mu.Lock()
	old, oldAt := known, at
	known, at = list, time.Now()
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		known, at = old, oldAt
		mu.Unlock()
	})
}

func TestAFolderOfTemplatesOffersEachFileInIt(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"Brief.odt": "odt", "Notiz.md": "# Notiz\n", ".versteckt": "x"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "Ordner"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := fromFolder(dir, "xdg:")
	if len(list) != 2 {
		t.Fatalf("templates = %+v", list)
	}
	use(t, list)

	out := filepath.Join(t.TempDir(), "Einkauf.md")
	if err := Make("xdg:Notiz.md", out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(out); string(got) != "# Notiz\n" {
		t.Errorf("new file holds %q", got)
	}
	if err := Make("xdg:Notiz.md", out); err == nil {
		t.Error("Make replaced a file that was there")
	}
	if err := Make("xdg:fehlt.md", out+"2"); !errors.Is(err, ErrUnknown) {
		t.Errorf("unknown template: %v", err)
	}
}

func TestATemplateOfWindowsWritesItsBytes(t *testing.T) {
	use(t, []Template{{ID: "shellnew:.zip", Name: "ZIP", Ext: ".zip", File: "ZIP", data: []byte("PK\x05\x06")}, {ID: "shellnew:.bmp", Name: "Bitmap", Ext: ".bmp", File: "Bitmap"}})
	dir := t.TempDir()
	if err := Make("shellnew:.zip", filepath.Join(dir, "a.zip")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.zip")); string(got) != "PK\x05\x06" {
		t.Errorf("zip holds %q", got)
	}
	if err := Make("shellnew:.bmp", filepath.Join(dir, "a.bmp")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(dir, "a.bmp")); info.Size() != 0 {
		t.Errorf("an empty template wrote %d bytes", info.Size())
	}
}
