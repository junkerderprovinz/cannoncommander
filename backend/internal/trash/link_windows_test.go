//go:build desktop

package trash

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// linkDir makes a junction, which any account may make without a privilege.
func linkDir(t *testing.T, target, at string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", at, target).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestATrashFolderOfAnotherAccountIsLeftAlone(t *testing.T) {
	root := t.TempDir()
	file(t, filepath.Join(root, DirName, "abc", "Dokumente.txt"), "fremd")
	if out, err := exec.Command("icacls", filepath.Join(root, DirName), "/setowner", "*S-1-5-18").CombinedOutput(); err != nil {
		t.Skipf("cannot hand the folder to SYSTEM here: %s", out)
	}
	s, bin := oldBin(t, root, "abc", "Dokumente.txt")
	s.takeOver()
	if len(bin.entries) != 0 {
		t.Fatalf("taken over: %+v", bin.entries)
	}
	if _, err := os.Stat(filepath.Join(root, DirName, "abc", "Dokumente.txt")); err != nil {
		t.Fatal("the file went away")
	}
}
