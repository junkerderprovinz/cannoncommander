package indexing

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// tree makes the files and folders under a fresh root, a name ending in /
// a folder, and hides the dotted ones the way the system does.
func tree(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		p := filepath.Join(root, filepath.FromSlash(n))
		var err error
		if n[len(n)-1] == '/' {
			err = os.MkdirAll(p, 0o755)
		} else if err = os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
			err = os.WriteFile(p, nil, 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
		hideDotted(t, root, n)
	}
	return root
}

// hideDotted hides every part of rel whose name starts with a dot, as the
// tests name what they mean to be hidden.
func hideDotted(t *testing.T, root, rel string) {
	t.Helper()
	p := root
	for _, part := range strings.Split(strings.Trim(rel, "/"), "/") {
		p = filepath.Join(p, part)
		if strings.HasPrefix(part, ".") {
			hide(t, p, false)
		}
	}
}

func TestIsHiddenFollowsTheRuleOfTheSystem(t *testing.T) {
	root := tree(t, "notes.txt", "Thumbs.db", "boot.ini")
	if err := os.WriteFile(filepath.Join(root, ".bashrc"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hide(t, filepath.Join(root, "Thumbs.db"), false)
	setAttrs(t, filepath.Join(root, "boot.ini"), 0x4)

	windows := runtime.GOOS == "windows"
	for name, want := range map[string]bool{"notes.txt": false, ".bashrc": !windows, "Thumbs.db": windows, "boot.ini": false} {
		if got := IsHidden(filepath.Join(root, name)); got != want {
			t.Errorf("IsHidden(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestTheTrashOfTheAppStaysOutOfTheIndexEverywhere(t *testing.T) {
	root := t.TempDir()
	trash := filepath.Join(root, ".cc-trash")
	if err := os.Mkdir(trash, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsHidden(trash) {
		t.Error("the trash folder counts as visible")
	}
}
