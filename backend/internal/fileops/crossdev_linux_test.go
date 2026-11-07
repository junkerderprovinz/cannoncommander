package fileops_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
)

func deviceOf(t *testing.T, p string) uint64 {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(p, &st); err != nil {
		t.Fatal(err)
	}
	return uint64(st.Dev)
}

func TestMoveAcrossFileSystemsTakesAFileHiddenByNameAlong(t *testing.T) {
	shm, err := os.MkdirTemp("/dev/shm", "cc-test-")
	if err != nil {
		t.Skip("no /dev/shm")
	}
	t.Cleanup(func() { os.RemoveAll(shm) })
	e := newEnv(t, ccfs.Source{Name: "shm", Mount: "/remotes/shm", Root: shm})
	if deviceOf(t, e.root) == deviceOf(t, shm) {
		t.Skip("the temp folder and /dev/shm share a file system")
	}
	album := albumWithThumbs(e)

	j := e.run("move", fileops.TransferParams{Sources: []string{album}, Dest: "/remotes/shm"})
	if !e.gone(album) {
		t.Fatal("the hidden file kept the source alive")
	}
	if b, err := os.ReadFile(filepath.Join(shm, "album", "sub", "Thumbs.db")); err != nil || string(b) != "klein" {
		t.Fatalf("moved file: %q, %v", b, err)
	}

	e.undo(j.Undo)
	if e.read(album+"/sub/Thumbs.db") != "klein" {
		t.Fatal("undo left the hidden file behind")
	}
}

func TestMoveAcrossFileSystemsCopiesAndRemovesTheSource(t *testing.T) {
	shm, err := os.MkdirTemp("/dev/shm", "cc-test-")
	if err != nil {
		t.Skip("no /dev/shm")
	}
	t.Cleanup(func() { os.RemoveAll(shm) })
	e := newEnv(t, ccfs.Source{Name: "shm", Mount: "/remotes/shm", Root: shm})
	if deviceOf(t, e.root) == deviceOf(t, shm) {
		t.Skip("the temp folder and /dev/shm share a file system")
	}
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "sonne", old)
	e.write("/mnt/user/fotos/Urlaub/2025/b.jpg", "strand", old)

	j := e.run("move", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/remotes/shm"})
	if !e.gone("/mnt/user/fotos/Urlaub") {
		t.Fatal("the source is still there")
	}
	b, err := os.ReadFile(filepath.Join(shm, "Urlaub", "2025", "b.jpg"))
	if err != nil || string(b) != "strand" {
		t.Fatalf("moved file: %q, %v", b, err)
	}
	if info, _ := os.Stat(filepath.Join(shm, "Urlaub", "a.jpg")); !info.ModTime().Equal(old) {
		t.Fatalf("mtime %s", info.ModTime())
	}

	e.undo(j.Undo)
	if e.read("/mnt/user/fotos/Urlaub/2025/b.jpg") != "strand" {
		t.Fatal("undo did not bring it back")
	}
	if _, err := os.Stat(filepath.Join(shm, "Urlaub")); !os.IsNotExist(err) {
		t.Fatal("undo left the copy behind")
	}
}
