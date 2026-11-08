package elevate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheFoldersAboveStayPutWhileTheShellRecycles(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "b", "c.txt"), "c")
	var renamed []error
	was := put
	put = func(path string) error {
		for _, p := range []string{filepath.Join(dir, "a"), filepath.Join(dir, "a", "b")} {
			renamed = append(renamed, os.Rename(p, p+"-weg"))
		}
		return nil
	}
	t.Cleanup(func() { put = was })
	if err := recycle(filepath.Join(dir, "a", "b", "c.txt")); err != nil {
		t.Fatal(err)
	}
	for i, err := range renamed {
		if err == nil {
			t.Errorf("folder %d was renamed during the call", i)
		}
	}
	if err := os.Rename(filepath.Join(dir, "a"), filepath.Join(dir, "a-weg")); err != nil {
		t.Fatalf("the folders stay held: %v", err)
	}
}

func TestTheShellRecyclesNothingBehindALink(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "ziel", "c.txt"), "c")
	linkDir(t, filepath.Join(dir, "ziel"), filepath.Join(dir, "link"))
	called := false
	was := put
	put = func(string) error { called = true; return nil }
	t.Cleanup(func() { put = was })
	if err := recycle(filepath.Join(dir, "link", "c.txt")); err == nil || called {
		t.Fatalf("recycled through a link: %v, %v", err, called)
	}
}
