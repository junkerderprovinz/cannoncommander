//go:build unix

package unraid

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func (e *env) link(dev, from, to string) string {
	e.t.Helper()
	p := filepath.Join(e.cfg.Mnt, dev, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.Link(filepath.Join(e.cfg.Mnt, dev, filepath.FromSlash(from)), p); err != nil {
		e.t.Fatal(err)
	}
	return "/mnt/user/" + to
}

func TestWhereFindsTheOtherNamesOfAHardLink(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "downloads/Big.Buck.Bunny.2008.1080p.mkv", 4)
	film := e.link("disk1", "downloads/Big.Buck.Bunny.2008.1080p.mkv", "filme/Big Buck Bunny (2008)/Big Buck Bunny (2008).mkv")
	w, err := e.svc.Where(context.Background(), src, false)
	if err != nil {
		t.Fatal(err)
	}
	if w.Links == nil || w.Links.Count != 2 || !w.Links.Complete || !slices.Equal(w.Links.Paths, []string{film}) {
		t.Fatalf("links = %+v", w.Links)
	}
	w, _ = e.svc.Where(context.Background(), "/mnt/disk1/filme/Big Buck Bunny (2008)/Big Buck Bunny (2008).mkv", false)
	if w.Links == nil || !slices.Equal(w.Links.Paths, []string{src}) {
		t.Errorf("through the disk share: links = %+v", w.Links)
	}
}

func TestPlainFileHasNoLinks(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk2", "musik/album.flac", 1)
	if w, _ := e.svc.Where(context.Background(), src, false); w.Links != nil {
		t.Errorf("links = %+v", w.Links)
	}
}

func TestCopyAndCrossShareMoveBreakHardLinks(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "downloads/album.flac", 3)
	e.link("disk1", "downloads/album.flac", "medien/album.flac")
	e.file("disk1", "downloads/single.flac", 1)

	rep := checkInto(t, e, "/mnt/user/backups", "copy", "/mnt/user/downloads")
	if rep.Links == nil || rep.Links.Count != 1 || rep.Links.Extra != 3*1024 {
		t.Fatalf("copy: links = %+v", rep.Links)
	}
	lf := rep.Links.Files[0]
	if lf.Path != src || lf.Names != 2 || !slices.Equal(lf.Others, []string{"/mnt/user/medien/album.flac"}) {
		t.Errorf("copy: file = %+v", lf)
	}
	if rep := checkInto(t, e, "/mnt/user/medien", "move", src); rep.Links == nil {
		t.Error("a move into another share keeps the links")
	}
	if rep := checkInto(t, e, "/mnt/user/downloads/neu", "move", src); rep.Links != nil {
		t.Errorf("a move inside the share breaks links: %+v", rep.Links)
	}
	if rep := checkInto(t, e, "/mnt/disk1/downloads/neu", "move", "/mnt/disk1/downloads/album.flac"); rep.Links != nil {
		t.Errorf("a move on the same disk breaks links: %+v", rep.Links)
	}
	if rep := checkInto(t, e, "/mnt/disk2/downloads", "move", "/mnt/disk1/downloads/album.flac"); rep.Links == nil {
		t.Error("a move to another disk keeps the links")
	}
	if rep := checkInto(t, e, "/mnt/user/filme", "link", src); rep.Links != nil || !rep.Linkable {
		t.Errorf("link: %+v", rep)
	}
}

type fakeIndex map[uint64][]string

func (f fakeIndex) PathsByInode(_ string, ino uint64) ([]string, bool) {
	rels, ok := f[ino]
	return rels, ok
}

func TestInodeIndexSavesTheWalk(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "downloads/a.mkv", 1)
	e.link("disk1", "downloads/a.mkv", "filme/a.mkv")
	info, err := os.Stat(filepath.Join(e.cfg.Mnt, "disk1", "downloads", "a.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	ino := inodeOf(info).ino
	e.svc.SetInodeIndex(fakeIndex{ino: {"downloads/a.mkv", "serien/a.mkv"}})
	w, _ := e.svc.Where(context.Background(), src, false)
	if w.Links == nil || !slices.Equal(w.Links.Paths, []string{"/mnt/user/serien/a.mkv"}) {
		t.Errorf("links = %+v, want what the index says", w.Links)
	}
}
