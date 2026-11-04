package unraid

import (
	"context"
	"testing"
)

func checkInto(t *testing.T, e *env, dest string, kind string, paths ...string) TransferReport {
	t.Helper()
	rep, err := e.svc.CheckTransfer(context.Background(), Transfer{Kind: kind, Paths: paths, Dest: dest})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestPickFollowsTheAllocationMethod(t *testing.T) {
	e := newEnv(t)
	st := e.state()
	cases := map[string]string{
		// Half of 4 MiB is more than anyone has free, at 1 MiB disk3 is the
		// first to reach the mark.
		"medien":  "disk3",
		"filme":   "disk1",
		"backups": "disk3",
		"voll":    "",
	}
	for name, want := range cases {
		sh, _ := st.Share(name)
		if got := pick(st, sh, sh.Disks); got != want {
			t.Errorf("%s (%s): picked %q, want %q", name, sh.Allocator, got, want)
		}
	}
}

func TestHighWaterSkipsDisksBelowTheMark(t *testing.T) {
	e := newEnv(t)
	st := e.state()
	sh, _ := st.Share("medien")
	if got := pick(st, sh, []string{"disk1", "disk2"}); got != "disk2" {
		t.Errorf("picked %q, want disk2 with 900 KiB over disk1 with 48", got)
	}
}

func TestSpaceWarnsWhenTheBiggestFileDoesNotFitThePickedDisk(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk3", "downloads/Sintel.mkv", 50)
	e.dir("disk1", "filme")
	rep := checkInto(t, e, "/mnt/user/filme", "copy", src)
	s := rep.Space
	if s == nil || s.Problem != SpaceBigFile || s.Disk != "disk1" || s.Roomy != "disk3" || s.Biggest.Size != 50*1024 {
		t.Fatalf("space = %+v", s)
	}
	if s.Free != 48*1024 || s.Floor != 30*1024 {
		t.Errorf("free %d floor %d", s.Free, s.Floor)
	}
}

func TestSpaceProblems(t *testing.T) {
	cases := []struct {
		share string
		files []int64
		want  string
		lands string
	}{
		{"backups", []int64{1000, 1000}, SpaceTooMuch, "disk3"},
		{"voll", []int64{1}, SpaceNoDisk, ""},
		{"appdata", []int64{300}, SpacePoolFull, "cache"},
		{"medien", []int64{100}, "", "disk3"},
		{"downloads", []int64{100}, "", "cache"},
		{"downloads", []int64{300}, "", "disk3"},
	}
	for _, c := range cases {
		e := newEnv(t)
		var paths []string
		for i, kib := range c.files {
			paths = append(paths, e.file("disk2", "quelle/"+string(rune('a'+i)), kib))
		}
		rep := checkInto(t, e, "/mnt/user/"+c.share, "copy", paths...)
		got := ""
		if rep.Space != nil {
			got = rep.Space.Problem
		}
		if got != c.want || rep.Lands != c.lands {
			t.Errorf("%s with %v KiB: problem %q lands %q, want %q on %q", c.share, c.files, got, rep.Lands, c.want, c.lands)
		}
	}
}

func TestSplitLevelKeepsAFolderOnItsDisk(t *testing.T) {
	e := newEnv(t)
	// serien splits only the share folder, so a season folder that already
	// exists on disk2 pins new episodes there although disk3 has more room.
	e.file("disk2", "serien/Show/S01E01.mkv", 1)
	src := e.file("disk3", "downloads/S01E02.mkv", 1)
	if rep := checkInto(t, e, "/mnt/user/serien/Show", "copy", src); rep.Lands != "disk2" {
		t.Errorf("lands on %q, want disk2", rep.Lands)
	}
	if rep := checkInto(t, e, "/mnt/user/serien", "copy", src); rep.Lands != "disk3" {
		t.Errorf("the share folder itself may split, lands on %q", rep.Lands)
	}
}

func TestMoveInsideAShareNeedsNoSpace(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk2", "filme/alt/Sintel.mkv", 500)
	if rep := checkInto(t, e, "/mnt/user/filme", "move", src); rep.Space != nil || rep.Lands != "" {
		t.Errorf("space = %+v lands %q", rep.Space, rep.Lands)
	}
}

func TestTransferNamesTheDisksItWakes(t *testing.T) {
	e := newEnv(t)
	src := e.file("disk1", "downloads/a.iso", 1)
	rep := checkInto(t, e, "/mnt/disk2/isos", "copy", src)
	if len(rep.Wakes) != 1 || rep.Wakes[0] != "disk1" {
		t.Errorf("wakes = %v", rep.Wakes)
	}
	e.writeDisks(map[string]bool{"disk1": true})
	if rep := checkInto(t, e, "/mnt/disk2/isos", "copy", src); len(rep.Wakes) != 0 {
		t.Errorf("an awake disk still wakes: %v", rep.Wakes)
	}
}
