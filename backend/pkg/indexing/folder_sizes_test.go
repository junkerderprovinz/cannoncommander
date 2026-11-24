package indexing

import (
	"maps"
	"testing"
)

func sizesOf(t *folderSizes) map[string]uint64 {
	out := map[string]uint64{}
	t.each(func(id uint32) { out[t.path(id)] = t.nodes[id].size })
	return out
}

func TestFolderSizesKeepWhatIsSet(t *testing.T) {
	f := newFolderSizes()
	f.set("/", 30)
	f.set("/a/", 20)
	f.set("/a/b/c/", 5)
	f.set("/a/b/c/", 7)
	if got, ok := f.get("/a/b/c/"); !ok || got != 7 {
		t.Errorf("/a/b/c/ = %d, %v", got, ok)
	}
	if got, ok := f.get("/a/b/c"); !ok || got != 7 {
		t.Errorf("without the slash: %d, %v", got, ok)
	}
	if _, ok := f.get("/a/b/"); ok {
		t.Error("a folder above one that was set has no size of its own")
	}
	if _, ok := f.get("/x/"); ok {
		t.Error("an unknown folder has a size")
	}
	want := map[string]uint64{"/": 30, "/a/": 20, "/a/b/c/": 7}
	if got := sizesOf(f); !maps.Equal(got, want) {
		t.Errorf("sizes %v, want %v", got, want)
	}
	if f.len() != 3 {
		t.Errorf("len %d, want 3", f.len())
	}
}

func TestRemovedFoldersKeepTheFoldersBelow(t *testing.T) {
	f := newFolderSizes()
	f.set("/a/", 2)
	f.set("/a/b/", 1)
	if size, ok := f.remove("/a/"); !ok || size != 2 {
		t.Errorf("remove gave %d, %v", size, ok)
	}
	if _, ok := f.remove("/a/"); ok {
		t.Error("a folder was removed twice")
	}
	if got := sizesOf(f); !maps.Equal(got, map[string]uint64{"/a/b/": 1}) {
		t.Errorf("sizes %v", got)
	}
}

func TestFoldersBelowAFolder(t *testing.T) {
	f := newFolderSizes()
	for _, p := range []string{"/a/", "/a/b/", "/a/b/c/", "/ab/", "/d/"} {
		f.set(p, 1)
	}
	var got []string
	f.eachBelow([]string{"/a/"}, func(id uint32) { got = append(got, f.path(id)) })
	if want := []string{"/a/", "/a/b/", "/a/b/c/"}; !equalSets(got, want) {
		t.Errorf("below /a/: %v, want %v", got, want)
	}
	got = nil
	f.eachBelow([]string{"/nowhere/"}, func(id uint32) { got = append(got, f.path(id)) })
	if len(got) != 0 {
		t.Errorf("below an unknown folder: %v", got)
	}
	got = nil
	f.eachBelow([]string{"/a/b/", "/d/"}, func(id uint32) { got = append(got, f.path(id)) })
	if want := []string{"/a/b/", "/a/b/c/", "/d/"}; !equalSets(got, want) {
		t.Errorf("below two folders: %v, want %v", got, want)
	}
}

func TestTheUnsyncedMarkFollowsChanges(t *testing.T) {
	f := newFolderSizes()
	f.set("/a/", 1)
	f.restore("/b/", 2)
	var dirty []string
	f.eachUnsynced(func(id uint32) { dirty = append(dirty, f.path(id)) })
	if !equalSets(dirty, []string{"/a/"}) {
		t.Errorf("unsynced %v, want only the folder that was set", dirty)
	}
	id, _ := f.lookup("/a/")
	f.synced(id)
	dirty = nil
	f.eachUnsynced(func(id uint32) { dirty = append(dirty, f.path(id)) })
	if len(dirty) != 0 {
		t.Errorf("unsynced after the sync: %v", dirty)
	}
}

func TestACompactedTreeHoldsTheSameSizes(t *testing.T) {
	f := newFolderSizes()
	for _, p := range []string{"/a/", "/a/b/", "/a/b/c/", "/d/", "/d/e/"} {
		f.set(p, uint64(len(p)))
	}
	f.remove("/a/b/c/")
	f.remove("/d/")
	f.remove("/d/e/")
	before := sizesOf(f)
	f.compact()
	if got := sizesOf(f); !maps.Equal(got, before) {
		t.Errorf("after compacting %v, want %v", got, before)
	}
	if len(f.nodes) != 3 {
		t.Errorf("%d nodes left, want the root, /a/ and /a/b/", len(f.nodes))
	}
	f.set("/d/e/", 9)
	if got, ok := f.get("/d/e/"); !ok || got != 9 {
		t.Errorf("a folder set after compacting: %d, %v", got, ok)
	}
}

func TestNamesThatShareAKeyStayApart(t *testing.T) {
	f := newFolderSizes()
	f.set("/b/", 1)
	b, _ := f.lookup("/b/")
	// A second name taking the key of /b/ is what a hash collision looks like.
	f.kids[f.key(0, "c")] = b
	f.set("/c/", 2)
	if got, ok := f.get("/b/"); !ok || got != 1 {
		t.Errorf("/b/ = %d, %v", got, ok)
	}
	if got, ok := f.get("/c/"); !ok || got != 2 {
		t.Errorf("/c/ = %d, %v", got, ok)
	}
	f.compact()
	if got := sizesOf(f); !maps.Equal(got, map[string]uint64{"/b/": 1, "/c/": 2}) {
		t.Errorf("after compacting %v", got)
	}
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
