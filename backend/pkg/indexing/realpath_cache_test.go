package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTheRealPathCacheDropsTheLeastRecentlyUsed(t *testing.T) {
	c := newRealPathCache(3, time.Hour)
	for _, k := range []string{"a", "b", "c"} {
		c.set(k, "/real/"+k, false)
	}
	c.get("a")
	c.set("d", "/real/d", true)
	if n := c.len(); n != 3 {
		t.Errorf("%d entries, want 3", n)
	}
	if _, _, ok := c.get("b"); ok {
		t.Error("b, used least recently, is still there")
	}
	for _, k := range []string{"a", "c", "d"} {
		if p, _, ok := c.get(k); !ok || p != "/real/"+k {
			t.Errorf("%s: %q, %v", k, p, ok)
		}
	}
	if _, isDir, _ := c.get("d"); !isDir {
		t.Error("d lost its folder flag")
	}
}

func TestARealPathExpires(t *testing.T) {
	c := newRealPathCache(3, -time.Second)
	c.set("a", "/real/a", false)
	if _, _, ok := c.get("a"); ok {
		t.Error("an expired path was returned")
	}
	if n := c.len(); n != 0 {
		t.Errorf("%d entries after the expired one was asked for, want 0", n)
	}
}

func TestLookupsAcrossAShareStayWithinTheLimit(t *testing.T) {
	old := realPaths
	realPaths = newRealPathCache(4, time.Hour)
	t.Cleanup(func() { realPaths = old })

	root := t.TempDir()
	idx := symlinkTestIndex(root)
	for i := range 10 {
		name := fmt.Sprintf("folder-%d", i)
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		got, isDir, err := idx.GetRealPath("/" + name)
		if err != nil || !isDir {
			t.Fatalf("%s: %q, %v, %v", name, got, isDir, err)
		}
		if want, _ := filepath.EvalSymlinks(filepath.Join(root, name)); got != want && got != filepath.Join(root, name) {
			t.Errorf("%s resolved to %q", name, got)
		}
	}
	if n := realPaths.len(); n > 4 {
		t.Errorf("the cache holds %d paths, its limit is 4", n)
	}
}
