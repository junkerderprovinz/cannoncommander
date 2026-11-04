//go:build !windows

package dav_test

import (
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestNewEntriesFollowTheUmaskAndReplacedFilesKeepTheirMode(t *testing.T) {
	old := syscall.Umask(0o027)
	t.Cleanup(func() { syscall.Umask(old) })
	e := newEnv(t)

	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/neu.jpg", "x", nil), http.StatusCreated)
	if info, _ := os.Stat(e.real("fotos/neu.jpg")); info.Mode().Perm() != 0o640 {
		t.Fatalf("new file mode %o", info.Mode().Perm())
	}
	e.want(e.ask("anna", "MKCOL", "/mnt/user/ordner", "", nil), http.StatusCreated)
	if info, _ := os.Stat(e.real("ordner")); info.Mode().Perm() != 0o750 {
		t.Fatalf("new folder mode %o", info.Mode().Perm())
	}

	if err := os.Chmod(e.real("fotos/a.jpg"), 0o604); err != nil {
		t.Fatal(err)
	}
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/a.jpg", "regen", nil), http.StatusCreated)
	if info, _ := os.Stat(e.real("fotos/a.jpg")); info.Mode().Perm() != 0o604 {
		t.Fatalf("replaced file mode %o", info.Mode().Perm())
	}
}

func TestMoveAcrossFileSystemsCopiesAndRemovesTheSource(t *testing.T) {
	shm, err := os.MkdirTemp("/dev/shm", "cc-test-")
	if err != nil {
		t.Skip("no /dev/shm")
	}
	t.Cleanup(func() { os.RemoveAll(shm) })
	e := newEnvOn(t, shm)
	var a, b syscall.Stat_t
	if syscall.Stat(e.root, &a) != nil || syscall.Stat(shm, &b) != nil || a.Dev == b.Dev {
		t.Skip("the temp folder and /dev/shm share a file system")
	}
	e.write("fotos/2026/b.jpg", "wolke")
	e.label("/mnt/user/fotos/2026/b.jpg", "red")
	e.want(e.ask("admin", "MOVE", "/mnt/user/fotos", "", e.dest("/mnt/disk1/fotos")), http.StatusCreated)
	if e.exists("fotos") {
		t.Fatal("the source is still there")
	}
	got, err := os.ReadFile(filepath.Join(shm, "fotos", "2026", "b.jpg"))
	if err != nil || string(got) != "wolke" {
		t.Fatalf("moved file: %q, %v", got, err)
	}
	if e.colour("/mnt/disk1/fotos/2026/b.jpg") != "red" {
		t.Fatal("the label stayed behind")
	}
	entries, _ := os.ReadDir(shm)
	if len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

func TestReplacedFileKeepsItsOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to hand out files")
	}
	e := newEnv(t)
	if err := os.Chown(e.real("fotos/a.jpg"), 99, 100); err != nil {
		t.Fatal(err)
	}
	e.want(e.ask("anna", "PUT", "/mnt/user/fotos/a.jpg", "regen", nil), http.StatusCreated)
	var st syscall.Stat_t
	if err := syscall.Stat(e.real("fotos/a.jpg"), &st); err != nil || st.Uid != 99 || st.Gid != 100 {
		t.Fatalf("owner %d:%d, %v", st.Uid, st.Gid, err)
	}
}

func TestNamesTheArchivePathsWouldMisreadAreLeftOut(t *testing.T) {
	e := newEnv(t)
	e.write("fotos/doppel::punkt", "x")
	e.write("alben/doppel::punkt", "x")
	eq(t, e.list("anna", "/mnt/user/fotos"), "a.jpg")
	e.want(e.ask("anna", "DELETE", "/mnt/user/alben", "", nil), http.StatusNoContent)
}
