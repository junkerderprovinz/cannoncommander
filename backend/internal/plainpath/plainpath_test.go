package plainpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestALinkIsNoPlainEntry(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ziel")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	at := filepath.Join(dir, "link")
	linkDir(t, target, at)
	for p, want := range map[string]bool{target: true, at: false} {
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := !Link(fi, p); got != want {
			t.Errorf("%s plain: %v", p, got)
		}
	}
	if err := Check(at); err == nil {
		t.Error("the link passed")
	}
	if err := Check(target); err != nil {
		t.Error(err)
	}
}

func TestEveryFolderOnTheWayCounts(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ziel", "tief"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir(t, filepath.Join(dir, "ziel"), filepath.Join(dir, "link"))
	if err := Path(filepath.Join(dir, "link", "tief", "a.txt")); err == nil {
		t.Error("a link on the way passed")
	}
	if err := Path(filepath.Join(dir, "ziel", "tief", "fehlt", "a.txt")); err != nil {
		t.Errorf("a plain path with missing parts: %v", err)
	}
}

func TestWhatThisAccountMadeIsItsOwn(t *testing.T) {
	dir := t.TempDir()
	if mine, err := Mine(dir); err != nil || !mine {
		t.Fatalf("%v, %v", mine, err)
	}
}
