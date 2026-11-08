package openwith

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsOffersAProgramForText(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o666); err != nil {
		t.Fatal(err)
	}
	apps, err := shell{}.apps(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) == 0 {
		t.Skip("this Windows offers no program for text")
	}
	for i, a := range apps {
		if a.id == "" || a.name == "" {
			t.Errorf("unnamed: %+v", a)
		}
		if a.def && i != 0 {
			t.Errorf("the default %s is not first", a.name)
		}
	}
}

func TestAFileWithoutAnExtensionHasNoPrograms(t *testing.T) {
	apps, err := shell{}.apps(filepath.Join(t.TempDir(), "README"))
	if err != nil || len(apps) != 0 {
		t.Fatalf("got %v %v", apps, err)
	}
}

func TestAnIconOfWindowsBecomesAPicture(t *testing.T) {
	u, err := shell{}.icon(filepath.Join(os.Getenv("SystemRoot"), "System32", "shell32.dll") + ",3")
	if err != nil {
		t.Fatal(err)
	}
	if img := decodeDataURL(t, u); img.Bounds() != image.Rect(0, 0, iconSize, iconSize) {
		t.Fatalf("got %v", img.Bounds())
	}
	for _, where := range []string{"", "nothing", `C:\nowhere.exe,0`, "@{nothing}"} {
		if _, err := (shell{}).icon(where); err == nil {
			t.Errorf("%q made an icon", where)
		}
	}
}

func TestAProgramWindowsDoesNotOfferIsNotStarted(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := (shell{}).open(file, `C:\Windows\System32\cmd.exe`); !errors.Is(err, errGone) {
		t.Fatalf("got %v", err)
	}
}
