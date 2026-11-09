package fsapi

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

type fakeIndex struct {
	mu        sync.Mutex
	folders   map[string]fsindex.Folder
	totals    map[string][2]int64
	refreshed []string
}

func (f *fakeIndex) Folders(_ *ccfs.Source, dirs []string) map[string]fsindex.Folder {
	out := map[string]fsindex.Folder{}
	for _, d := range dirs {
		if v, ok := f.folders[d]; ok {
			out[d] = v
		}
	}
	return out
}

func (f *fakeIndex) FolderSize(_ *ccfs.Source, dir string) (int64, bool) {
	v, ok := f.folders[dir]
	return v.Size, ok
}

func (f *fakeIndex) Totals(_ *ccfs.Source, dir string) (int64, int64, bool) {
	t, ok := f.totals[dir]
	return t[0], t[1], ok
}

func (f *fakeIndex) Refresh(_ *ccfs.Source, dir string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshed = append(f.refreshed, dir)
}

func (f *fakeIndex) Created(*ccfs.Source, string)       {}
func (f *fakeIndex) Moved(*ccfs.Source, string, string) {}
func (f *fakeIndex) Removed(*ccfs.Source, string)       {}

func byName(t *testing.T, items []Item, name string) Item {
	t.Helper()
	for _, it := range items {
		if it.Name == name {
			return it
		}
	}
	t.Fatalf("%s is not in %v", name, names(items))
	return Item{}
}

func TestListCountsFoldersAndFindsTheirPicture(t *testing.T) {
	f := newFixture(t)
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/filme"))
	sintel := byName(t, l.Items, "Sintel (2010)")
	if sintel.Items == nil || *sintel.Items != 2 || sintel.Picture != "poster.jpg" {
		t.Fatalf("folder = %+v", sintel)
	}
	l = decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/filme/Sintel (2010)"))
	if film := byName(t, l.Items, "Sintel (2010).mkv"); film.Format != "video" || film.Items != nil {
		t.Errorf("film = %+v", film)
	}
	if pic := byName(t, l.Items, "poster.jpg"); pic.Format != "image" {
		t.Errorf("poster = %+v", pic)
	}
	musik := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/musik"))
	if podcasts := byName(t, musik.Items, "Podcasts"); podcasts.Items == nil || *podcasts.Items != 0 || podcasts.Picture != "" {
		t.Errorf("empty folder = %+v", podcasts)
	}
}

func TestListTakesFolderSizesFromTheIndex(t *testing.T) {
	idx := &fakeIndex{folders: map[string]fsindex.Folder{
		"/filme/Sintel (2010)": {Items: 7, Size: 123456, Pictures: []string{"cover.jpg", "Folder.jpg"}},
	}}
	f := newFixture(t, idx)
	sintel := byName(t, decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/filme")).Items, "Sintel (2010)")
	if *sintel.Items != 7 || sintel.Size != 123456 || sintel.Picture != "Folder.jpg" {
		t.Fatalf("folder = %+v", sintel)
	}

	idx.folders["/filme/Sintel (2010)"] = fsindex.Folder{Items: 2, Size: -1}
	sintel = byName(t, decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/filme")).Items, "Sintel (2010)")
	if *sintel.Items != 2 || sintel.Size != 0 || !sintel.SizeUnknown {
		t.Fatalf("folder without a measured size = %+v", sintel)
	}
}

func TestListTellsAnEmptyFolderFromOneNotMeasuredYet(t *testing.T) {
	idx := &fakeIndex{folders: map[string]fsindex.Folder{
		"/musik/Podcasts": {Items: 0, Size: 0},
		"/musik/Alben":    {Items: 4, Size: -1},
	}}
	f := newFixture(t, idx)
	write(t, filepath.Join(f.share, "musik", "Alben", "a.flac"), 3)
	write(t, filepath.Join(f.share, "musik", "Hörbücher", "b.mp3"), 3)
	write(t, filepath.Join(f.share, "musik", "c.mp3"), 3)
	items := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/musik")).Items
	if empty := byName(t, items, "Podcasts"); empty.SizeUnknown {
		t.Errorf("measured empty folder = %+v", empty)
	}
	for _, name := range []string{"Alben", "Hörbücher"} {
		if k := byName(t, items, name); !k.SizeUnknown {
			t.Errorf("%s has no size from the index but = %+v", name, k)
		}
	}
	if file := byName(t, items, "c.mp3"); file.SizeUnknown {
		t.Errorf("file = %+v", file)
	}
}

func TestStatMarksAFolderNotMeasuredYet(t *testing.T) {
	f := newFixture(t, &fakeIndex{})
	if st := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/filme")); !st.SizeUnknown {
		t.Fatalf("folder = %+v", st)
	}
}

func TestListMarksEntriesThatLiveOnAPool(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.cache, "downloads", "setup-notes.md"), 4)
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/downloads"))
	if notes := byName(t, l.Items, "setup-notes.md"); notes.Pool != "cache" {
		t.Errorf("notes = %+v", notes)
	}
	if partial := byName(t, l.Items, ".partial-download"); partial.Pool != "" {
		t.Errorf("partial = %+v", partial)
	}
	shares := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user"))
	if d := byName(t, shares.Items, "downloads"); d.Pool != "cache" {
		t.Errorf("share = %+v", d)
	}
	if d := byName(t, shares.Items, "filme"); d.Pool != "" {
		t.Errorf("share without cache = %+v", d)
	}
}

func TestListReportsInodeAndOwner(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("inode details come from statx")
	}
	f := newFixture(t)
	notes := filepath.Join(f.share, "downloads", "setup-notes.md")
	if err := os.Link(notes, filepath.Join(f.share, "downloads", "zweiter-name.md")); err != nil {
		t.Fatal(err)
	}
	l := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/downloads"))
	a := byName(t, l.Items, "setup-notes.md")
	if a.Ino != 0 || a.Nlink != 2 {
		t.Errorf("share entry %+v should have links but no inode of its own", a)
	}
	if a.UID == nil || a.GID == nil || *a.UID != uint32(os.Getuid()) || a.Owner == "" {
		t.Errorf("owner of %+v", a)
	}
}

func TestListShowsSpecialModeBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no setgid on Windows")
	}
	f := newFixture(t)
	dir := filepath.Join(f.share, "musik", "Podcasts")
	if err := os.Chmod(dir, 0o775|os.ModeSetgid); err != nil {
		t.Fatal(err)
	}
	podcasts := byName(t, decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/musik")).Items, "Podcasts")
	if podcasts.Mode != "2775" {
		t.Fatalf("mode = %q", podcasts.Mode)
	}
}

func TestListAndTreeMarkRemotes(t *testing.T) {
	SetRemotes(func(p string) string {
		if p == "/mnt/user/downloads" {
			return "gdrive:"
		}
		return ""
	})
	t.Cleanup(func() { SetRemotes(nil) })
	f := newFixture(t)
	if d := byName(t, decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user")).Items, "downloads"); d.Remote != "gdrive:" {
		t.Errorf("list item = %+v", d)
	}
	tr := decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/mnt/user"))
	for _, it := range tr.Items {
		want := ""
		if it.Name == "downloads" {
			want = "gdrive:"
		}
		if it.Remote != want {
			t.Errorf("tree %s remote = %q", it.Name, it.Remote)
		}
	}
}

func TestADriveInsideASourceGoesByItsOwnName(t *testing.T) {
	f := newFixture(t)
	f.fs.SetVolumes([]ccfs.Volume{{Name: "ccshare", Root: filepath.Join(f.share, "fotos"), Drive: ccfs.Drive{Label: "ccshare", Kind: "network", Remote: "smb://localhost/ccshare"}}})
	got := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos/2025")).Volume
	if got == nil || got.Path != "/mnt/user/fotos" || got.Label != "ccshare" || got.Kind != "network" || got.Remote != "smb://localhost/ccshare" {
		t.Errorf("volume inside the drive = %+v", got)
	}
	if other := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/filme")).Volume; other == nil || other.Label != "Shares" || other.Kind != "" {
		t.Errorf("volume beside it = %+v", other)
	}
	fotos := byTreeName(t, decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/mnt/user")), "fotos")
	if fotos.Label != "ccshare" || fotos.Kind != "network" || fotos.Remote != "smb://localhost/ccshare" {
		t.Errorf("tree item = %+v", fotos)
	}
}

func byTreeName(t *testing.T, tr Tree, name string) TreeItem {
	t.Helper()
	for _, it := range tr.Items {
		if it.Name == name {
			return it
		}
	}
	t.Fatalf("no %s in %+v", name, tr.Items)
	return TreeItem{}
}

func TestStatDescribesFilesAndFolders(t *testing.T) {
	idx := &fakeIndex{totals: map[string][2]int64{"/filme": {3, 1}}}
	f := newFixture(t, idx)
	st := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/filme"))
	if st.Path != "/mnt/user/filme" || st.Name != "filme" || !st.Dir || st.Items == nil || *st.Items != 1 {
		t.Fatalf("folder = %+v", st)
	}
	if st.Files == nil || *st.Files != 3 || *st.Dirs != 1 || st.Volume == nil || st.Volume.Label != "Shares" {
		t.Errorf("folder totals = %+v", st)
	}
	file := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/downloads/setup-notes.md"))
	if file.Dir || file.Size != 4 || file.Format != "text" || file.Files != nil {
		t.Errorf("file = %+v", file)
	}
	root := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt"))
	if !root.Dir || root.Name != "mnt" || root.Volume != nil {
		t.Errorf("virtual folder = %+v", root)
	}
	share := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user"))
	if share.Name != "user" || !share.Dir {
		t.Errorf("source root = %+v", share)
	}
	if rec := f.get(t, "anna", "/fs/stat", "/mnt/user/filme"); rec.Code != 403 {
		t.Errorf("anna stats filme: %d", rec.Code)
	}
}

func TestFormatsMatchThePrototype(t *testing.T) {
	proto, err := os.ReadFile(filepath.Join("..", "..", "..", "prototype", "src", "formats.json"))
	if err != nil {
		t.Skip("prototype not checked out")
	}
	strip := func(b []byte) string { return strings.ReplaceAll(string(b), "\r", "") }
	if strip(proto) != strip(formatsJSON) {
		t.Fatal("internal/fsapi/formats.json differs from prototype/src/formats.json, copy it over")
	}
}

func TestFormatOfNames(t *testing.T) {
	cases := map[string]string{
		"Sintel (2010).mkv": "video",
		"IMG_0001.JPG":      "image",
		"syslog.log.1":      "log",
		"archiv.tar.zst":    "archive",
		".env":              "config",
		"README":            "",
		"film.mkv.part":     "temp",
	}
	for name, want := range cases {
		if got := FormatOf(name); got != want {
			t.Errorf("FormatOf(%q) = %q, want %q", name, got, want)
		}
	}
	if !slices.Contains(fsindex.FolderPictures, "poster.jpg") {
		t.Fatal("poster.jpg is not a folder picture")
	}
}

func TestStatFindsTheDisksOfASharePath(t *testing.T) {
	f := newFixture(t)
	base := filepath.Dir(f.share)
	write(t, filepath.Join(base, "disk2", "downloads", "setup-notes.md"), 4)
	write(t, filepath.Join(f.cache, "downloads", "setup-notes.md"), 4)
	write(t, filepath.Join(base, "disk1", "filme", "anderes.mkv"), 1)
	st := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/downloads/setup-notes.md"))
	if !slices.Equal(st.Devices, []string{"cache", "disk2"}) || st.Pool != "cache" {
		t.Fatalf("stat = %+v", st)
	}
	if runtime.GOOS == "linux" {
		disk, err := lstatFile(filepath.Join(f.cache, "downloads", "setup-notes.md"))
		if err != nil {
			t.Fatal(err)
		}
		if st.Ino != disk.ino {
			t.Errorf("inode %d, the cache has %d", st.Ino, disk.ino)
		}
	}
	if boot := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/boot/config/go")); boot.Devices != nil {
		t.Errorf("outside the shares: %+v", boot)
	}
}

func TestBookkeepingHidesTrashAndPartFiles(t *testing.T) {
	cases := map[string]bool{
		".cc-trash":         true,
		".film.mkv.cc-part": true,
		"film.mkv.cc-part":  false,
		".hidden":           false,
		"notes.cc-trash":    false,
	}
	for name, want := range cases {
		if got := bookkeeping(name); got != want {
			t.Errorf("bookkeeping(%q) = %v, want %v", name, got, want)
		}
	}
}
