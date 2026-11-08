package plainpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOnlyNameSurrogatesAreLinks(t *testing.T) {
	for tag, want := range map[uint32]bool{
		0xA0000003: true,  // mount point, a junction
		0xA000000C: true,  // symbolic link
		0xA000001D: true,  // WSL symlink
		0x9000001A: false, // OneDrive and other cloud files
		0x80000013: false, // deduplicated file
		0x8000001B: false, // app execution alias
	} {
		if got := nameSurrogate(tag); got != want {
			t.Errorf("%#x: %v", tag, got)
		}
	}
}

func TestAnAppExecutionAliasIsNoLink(t *testing.T) {
	aliases, _ := filepath.Glob(filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps", "*.exe"))
	if len(aliases) == 0 {
		t.Skip("no app execution alias here")
	}
	fi, err := os.Lstat(aliases[0])
	if err != nil {
		t.Fatal(err)
	}
	if Link(fi, aliases[0]) {
		t.Fatalf("%s counts as a link", aliases[0])
	}
}

func TestAFolderOfTheAdministratorsGroupCountsForAnAdmin(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("icacls", dir, "/setowner", "*S-1-5-32-544").CombinedOutput(); err != nil {
		t.Skipf("cannot hand the folder to Administrators here: %s", out)
	}
	if !admin() {
		t.Skip("this account is no administrator")
	}
	if mine, err := Mine(dir); err != nil || !mine {
		t.Fatalf("%v, %v", mine, err)
	}
}

func TestAFolderOfAnotherAccountIsNotMine(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("icacls", dir, "/setowner", "*S-1-5-18").CombinedOutput(); err != nil {
		t.Skipf("cannot hand the folder to SYSTEM here: %s", out)
	}
	if mine, err := Mine(dir); err != nil || mine {
		t.Fatalf("%v, %v", mine, err)
	}
}

func TestHeldFoldersCannotBeRenamed(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(dir, "a", "b", "c.txt")
	if err := os.WriteFile(item, []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := Hold(item)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "a"), filepath.Join(dir, "a", "b")} {
		if err := os.Rename(p, p+"-weg"); err == nil {
			t.Errorf("%s was renamed while held", p)
		}
	}
	if err := os.Rename(item, item+".neu"); err != nil {
		t.Errorf("the item itself is not held: %v", err)
	}
	release()
	if err := os.Rename(filepath.Join(dir, "a"), filepath.Join(dir, "a-weg")); err != nil {
		t.Fatalf("still held after release: %v", err)
	}
}

func TestHoldRefusesALinkOnTheWay(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ziel"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir(t, filepath.Join(dir, "ziel"), filepath.Join(dir, "link"))
	if release, err := Hold(filepath.Join(dir, "link", "a.txt")); err == nil {
		release()
		t.Fatal("held through a link")
	}
}
