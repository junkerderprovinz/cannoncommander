package winshell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func files(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	var out []string
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o666); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestANameLeadsBackToItsPath(t *testing.T) {
	p := files(t, t.TempDir(), "notes.txt")[0]
	l, err := ParseName(p)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Free()
	got, err := l.Path()
	if err != nil {
		t.Fatal(err)
	}
	// The temp folder may come back in its long or its short spelling.
	if !strings.EqualFold(filepath.Base(got), "notes.txt") {
		t.Fatalf("got %s", got)
	}
}

func TestAMissingFileHasNoName(t *testing.T) {
	if _, err := ParseName(filepath.Join(t.TempDir(), "gone.txt")); err == nil {
		t.Fatal("a missing file got a name")
	}
}

func TestItemsOfOneFolderMakeASelection(t *testing.T) {
	err := Apartment(func() error {
		s, err := Select(files(t, t.TempDir(), "a.txt", "b.txt"))
		if err != nil {
			return err
		}
		defer s.Close()
		if len(s.children) != 2 {
			t.Errorf("got %d children", len(s.children))
		}
		obj, err := s.DataObject()
		if err != nil {
			return err
		}
		Release(obj)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestItemsOfTwoFoldersAreNoSelection(t *testing.T) {
	a := files(t, t.TempDir(), "a.txt")
	b := files(t, t.TempDir(), "b.txt")
	err := Apartment(func() error {
		s, err := Select(append(a, b...))
		if err == nil {
			s.Close()
		}
		return err
	})
	if !errors.Is(err, ErrSeveralFolders) {
		t.Fatalf("got %v", err)
	}
}

func TestAnIconOfWindowsBecomesAPicture(t *testing.T) {
	img, err := ExtractIcon(filepath.Join(os.Getenv("SystemRoot"), "System32", "shell32.dll"), 3, 32)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("got %v", b)
	}
	if _, err := ExtractIcon(`C:\nowhere.exe`, 0, 32); err == nil {
		t.Fatal("a missing file gave an icon")
	}
}

func TestAnotherProgramsWindowOwnsNothing(t *testing.T) {
	if w := Owner(); w != 0 {
		var pid uint32
		if _, err := windows.GetWindowThreadProcessId(w, &pid); err != nil || int(pid) != os.Getpid() {
			t.Fatalf("owner %v belongs to %d", w, pid)
		}
	}
}
