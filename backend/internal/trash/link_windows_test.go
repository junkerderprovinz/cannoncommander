//go:build desktop

package trash

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
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

func TestATrashFolderOfTheAdministratorsIsTakenOverForAnAdmin(t *testing.T) {
	root := t.TempDir()
	file(t, filepath.Join(root, DirName, "abc", "Dokumente.txt"), "eigen")
	if out, err := exec.Command("icacls", filepath.Join(root, DirName), "/setowner", "*S-1-5-32-544", "/T").CombinedOutput(); err != nil {
		t.Skipf("cannot hand the folder to Administrators here: %s", out)
	}
	if !inAdministrators() {
		t.Skip("this account is no administrator")
	}
	s, bin := oldBin(t, root, "abc", "Dokumente.txt")
	s.takeOver()
	if len(bin.entries) != 1 {
		t.Fatalf("not taken over: %+v", bin.entries)
	}
}

func inAdministrators() bool {
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.AllGroups() {
		if g.Sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
			return true
		}
	}
	return false
}
