package sql

import (
	"path"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
)

func newFolderDB(t *testing.T, name string) *IndexDB {
	t.Helper()
	pop := pushTestIndexConfig(t, t.TempDir(), testIndexSQLConfig(""))
	t.Cleanup(pop)
	db, _, err := NewIndexDB(name, "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func entry(p string, dir bool, attrs iteminfo.UnixAttrs) *iteminfo.FileInfo {
	name := path.Base(p)
	typ := "text/plain"
	if dir {
		typ = "directory"
	}
	return &iteminfo.FileInfo{Path: p, ItemInfo: iteminfo.ItemInfo{Name: name, Type: typ, ModTime: time.Unix(1, 0), Unix: attrs}}
}

func TestIndexStoresInodeDetails(t *testing.T) {
	db := newFolderDB(t, "unix_attrs")
	attrs := iteminfo.UnixAttrs{Known: true, Inode: 4711, Nlink: 2, UID: 99, GID: 100, Mode: 0o2775}
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{entry("/a.mkv", false, attrs)}); err != nil {
		t.Fatal(err)
	}
	// A later write that knows nothing about the inode keeps what is stored.
	if err := db.InsertItem("src", "/a.mkv", entry("/a.mkv", false, iteminfo.UnixAttrs{})); err != nil {
		t.Fatal(err)
	}
	var ino, nlink, uid, gid, mode int64
	err := db.QueryRow(`SELECT ino, nlink, uid, gid, mode FROM index_items WHERE source = 'src' AND path = '/a.mkv'`).Scan(&ino, &nlink, &uid, &gid, &mode)
	if err != nil {
		t.Fatal(err)
	}
	if ino != 4711 || nlink != 2 || uid != 99 || gid != 100 || mode != 0o2775 {
		t.Fatalf("stored ino=%d nlink=%d uid=%d gid=%d mode=%o", ino, nlink, uid, gid, mode)
	}
}

func TestFolderQueriesCountWhatTheIndexHolds(t *testing.T) {
	db := newFolderDB(t, "folder_queries")
	items := []*iteminfo.FileInfo{
		entry("/filme/", true, iteminfo.UnixAttrs{}),
		entry("/filme/Sintel/", true, iteminfo.UnixAttrs{}),
		entry("/filme/Sintel/Sintel.mkv", false, iteminfo.UnixAttrs{}),
		entry("/filme/Sintel/Poster.JPG", false, iteminfo.UnixAttrs{}),
		entry("/filme/Sintel/extras/", true, iteminfo.UnixAttrs{}),
		entry("/filme/Leer/", true, iteminfo.UnixAttrs{}),
		entry("/musik/", true, iteminfo.UnixAttrs{}),
		entry("/musik/cover.jpg", false, iteminfo.UnixAttrs{}),
	}
	if err := db.BulkInsertItems("src", items); err != nil {
		t.Fatal(err)
	}

	counts, err := db.FolderCounts("src", []string{"/filme/Sintel/", "/filme/Leer/", "/filme/Unbekannt/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts["/filme/Sintel/"] != 3 || counts["/filme/Leer/"] != 0 {
		t.Fatalf("counts = %v", counts)
	}

	pics, err := db.FilesNamed("src", []string{"/filme/Sintel/", "/musik/", "/filme/Leer/"}, []string{"poster.jpg", "cover.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pics["/filme/Sintel/"], []string{"Poster.JPG"}) || !slices.Equal(pics["/musik/"], []string{"cover.jpg"}) || len(pics) != 2 {
		t.Fatalf("pictures = %v", pics)
	}

	files, dirs, err := db.TreeCounts("src", "/filme/")
	if err != nil {
		t.Fatal(err)
	}
	if files != 2 || dirs != 3 {
		t.Fatalf("below /filme/: %d files, %d folders", files, dirs)
	}
}

func TestFolderCountsTakeLongLists(t *testing.T) {
	db := newFolderDB(t, "folder_chunks")
	var items []*iteminfo.FileInfo
	var dirs []string
	for i := range 1200 {
		d := "/d" + strconv.Itoa(i) + "/"
		dirs = append(dirs, d)
		items = append(items, entry(d, true, iteminfo.UnixAttrs{}), entry(d+"x.txt", false, iteminfo.UnixAttrs{}))
	}
	if err := db.BulkInsertItems("src", items); err != nil {
		t.Fatal(err)
	}
	counts, err := db.FolderCounts("src", dirs)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1200 || counts["/d1199/"] != 1 {
		t.Fatalf("%d folders counted, d1199 = %d", len(counts), counts["/d1199/"])
	}
}
