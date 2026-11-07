package unraid

import (
	"context"
	"slices"
	"testing"
)

func TestTrapFromDiskShareIntoTheUserShareAbove(t *testing.T) {
	e := newEnv(t)
	e.file("disk2", "filme/Sintel (2010)/Sintel.mkv", 3)
	e.file("disk2", "filme/Sintel (2010)/poster.jpg", 1)
	rep := checkInto(t, e, "/mnt/user/filme", "copy", "/mnt/disk2/filme/Sintel (2010)")
	tr := rep.Trap
	if tr == nil || tr.Device != "disk2" || tr.Count != 2 || tr.Size != 4*1024 {
		t.Fatalf("trap = %+v", tr)
	}
	if tr.Files[0].Path != "/mnt/user/filme/Sintel (2010)/Sintel.mkv" && tr.Files[1].Path != "/mnt/user/filme/Sintel (2010)/Sintel.mkv" {
		t.Errorf("files = %+v", tr.Files)
	}
	var targets []string
	for _, d := range tr.Targets {
		targets = append(targets, d.Disk)
	}
	if !slices.Equal(targets, []string{"disk3", "disk1"}) {
		t.Errorf("targets = %v, want the other disks with the most free first", targets)
	}
}

func TestTrapFromUserShareOntoTheDiskHoldingTheFile(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "filme/Big Buck Bunny.mkv", 2)
	e.file("disk3", "filme/Tears of Steel.mkv", 2)
	rep := checkInto(t, e, "/mnt/disk1/filme", "move", src, "/mnt/user/filme/Tears of Steel.mkv")
	if rep.Trap == nil || rep.Trap.Count != 1 || rep.Trap.Files[0].Device != "disk1" {
		t.Fatalf("trap = %+v, only the file already on disk1 is caught", rep.Trap)
	}
}

func TestNoTrapBetweenDifferentFoldersOrOnOneSide(t *testing.T) {
	e := newEnv(t)
	e.file("disk2", "filme/a.mkv", 1)
	for _, c := range []struct{ src, dest string }{
		{"/mnt/disk2/filme/a.mkv", "/mnt/user/archiv"},
		{"/mnt/disk2/filme/a.mkv", "/mnt/disk3/filme"},
		{"/mnt/user/filme/a.mkv", "/mnt/user/filme"},
	} {
		if rep := checkInto(t, e, c.dest, "copy", c.src); rep.Trap != nil {
			t.Errorf("%s to %s: trap %+v", c.src, c.dest, rep.Trap)
		}
	}
}

func TestLinkableOnlyWhereTheFileCanStayOnItsDisk(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "downloads/Big Buck Bunny.mkv", 2)
	cached := e.file("cache", "downloads/album.flac", 1)
	cases := []struct {
		paths []string
		dest  string
		want  bool
	}{
		{[]string{src}, "/mnt/user/filme", true},
		{[]string{src}, "/mnt/user/backups", false},
		{[]string{src}, "/mnt/disk1/filme", true},
		{[]string{src}, "/mnt/disk2/filme", false},
		{[]string{cached}, "/mnt/user/appdata", true},
		{[]string{cached}, "/mnt/user/filme", false},
		{[]string{src}, "/mnt/user/downloads", false},
		{[]string{src}, "/mnt/user", false},
	}
	for _, c := range cases {
		if got := checkInto(t, e, c.dest, "copy", c.paths...).Linkable; got != c.want {
			t.Errorf("%v into %s: linkable %v, want %v", c.paths, c.dest, got, c.want)
		}
	}
}

func TestTransferOutsideUnraidReportsNothing(t *testing.T) {
	e := newEnv(t)
	rep, err := e.svc.CheckTransfer(context.Background(), Transfer{Kind: "copy", Paths: []string{"/remotes/nas/a"}, Dest: "/mnt/user/filme"})
	if err != nil || rep.Trap != nil || rep.Space != nil || rep.Links != nil || rep.Linkable || len(rep.Wakes) != 0 {
		t.Errorf("report = %+v, %v", rep, err)
	}
}

func TestWhereSumsAFolderPerDisk(t *testing.T) {
	e := newEnv(t)
	e.file("disk1", "filme/Tears of Steel/poster.jpg", 1)
	e.file("disk3", "filme/Tears of Steel/Tears of Steel.mkv", 10)
	e.file("disk3", "filme/Tears of Steel/extras/making-of.mkv", 5)
	w, err := e.svc.Where(context.Background(), "/mnt/user/filme/Tears of Steel", true)
	if err != nil {
		t.Fatal(err)
	}
	if w.Volume != "user" || w.Share != "filme" || w.Files != 3 || w.Size != 16*1024 || len(w.On) != 2 {
		t.Fatalf("where = %+v", w)
	}
	if w.On[0] != (Placement{Device: "disk1", Files: 1, Size: 1024, Asleep: true}) || w.On[1].Device != "disk3" || w.On[1].Size != 15*1024 {
		t.Errorf("on = %+v", w.On)
	}
	if got := e.svc.DevicesOf("/mnt/user/filme/Tears of Steel"); !slices.Equal(got, []string{"disk1", "disk3"}) {
		t.Errorf("DevicesOf = %v", got)
	}
}

func TestWhereShowsAnEmptyFolderOnItsDisk(t *testing.T) {
	e := newEnv(t)
	e.dir("disk2", "fotos/leer")
	w, err := e.svc.Where(context.Background(), "/mnt/user/fotos/leer", true)
	if err != nil || len(w.On) != 1 || w.On[0].Device != "disk2" || w.Files != 0 {
		t.Errorf("where = %+v, %v", w, err)
	}
}

func TestWhereOffTheArrayIsEmpty(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/remotes/nas", "/boot/config", "/mnt/user/a.zip::/b"} {
		w, err := e.svc.Where(context.Background(), p, false)
		if err != nil || w.Volume != "" || len(w.On) != 0 {
			t.Errorf("%s: %+v, %v", p, w, err)
		}
	}
}
