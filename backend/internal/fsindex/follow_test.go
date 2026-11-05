package fsindex_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

type calls struct {
	fsindex.Index
	seen []string
}

func (c *calls) Created(src *ccfs.Source, p string) { c.seen = append(c.seen, "created "+src.Name+p) }
func (c *calls) Removed(src *ccfs.Source, p string) { c.seen = append(c.seen, "removed "+src.Name+p) }
func (c *calls) Moved(src *ccfs.Source, from, to string) {
	c.seen = append(c.seen, "moved "+src.Name+from+" "+to)
}

func TestTheIndexFollowsWhatTheAppChanges(t *testing.T) {
	r, err := ccfs.New(nil,
		ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: t.TempDir()},
		ccfs.Source{Name: "Disk", Mount: "/mnt/disk1", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	c := &calls{}
	stop := fsindex.Follow(r, c)
	fschange.Created("/mnt/user/filme/neu.mkv")
	fschange.Removed("/mnt/user/filme/alt")
	fschange.Moved("/mnt/user/filme/a", "/mnt/user/serien/b")
	fschange.Moved("/mnt/user/filme/c", "/mnt/disk1/filme/c")
	fschange.Created("/mnt")
	stop()
	fschange.Created("/mnt/user/später")
	want := []string{
		"created Shares/filme/neu.mkv",
		"removed Shares/filme/alt",
		"moved Shares/filme/a /serien/b",
		"removed Shares/filme/c",
		"created Disk/filme/c",
	}
	if !slices.Equal(c.seen, want) {
		t.Fatalf("index heard %q, want %q", c.seen, want)
	}
}
