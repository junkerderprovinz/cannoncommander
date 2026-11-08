package elevate

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAMoveAcrossVolumesCopiesAndDeletes(t *testing.T) {
	var shm, tmp syscall.Stat_t
	if syscall.Stat("/dev/shm", &shm) != nil || syscall.Stat(os.TempDir(), &tmp) != nil || shm.Dev == tmp.Dev || syscall.Access("/dev/shm", 2) != nil {
		t.Skip("no second volume to move to")
	}
	src, err := os.MkdirTemp("/dev/shm", "cc-elevate-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(src) })
	dir := t.TempDir()
	file(t, filepath.Join(src, "a", "x.txt"), "x")
	if err := apply(Op{Kind: Move, From: filepath.Join(src, "a"), To: filepath.Join(dir, "a")}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "a", "x.txt")) != "x" {
		t.Fatal("not moved")
	}
	if _, err := os.Lstat(filepath.Join(src, "a")); err == nil {
		t.Fatal("the source is still there")
	}
}
