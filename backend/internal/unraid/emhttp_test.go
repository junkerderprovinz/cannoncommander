package unraid

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// testdataConfig points at the files taken from an Unraid 7.3.4 server.
func testdataConfig() Config {
	c := DefaultConfig()
	c.Emhttp = filepath.Join("testdata", "emhttp")
	c.Shares = filepath.Join("testdata", "shares")
	c.BootConfig = filepath.Join("testdata", "config")
	c.Mdstat = filepath.Join("testdata", "mdstat")
	return c
}

func readTestdata(t *testing.T, c Config) *State {
	t.Helper()
	st, err := ReadState(c)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestReadStateFromUnraid734(t *testing.T) {
	st := readTestdata(t, testdataConfig())
	if !st.Available || st.Version != "7.3.4" || st.Name != "Tower" {
		t.Fatalf("got available=%v version=%q name=%q", st.Available, st.Version, st.Name)
	}
	if !st.Array.Started || st.Array.Disks != 5 || st.Array.Unmountable != 1 {
		t.Errorf("array = %+v", st.Array)
	}
	var names []string
	for _, d := range st.Disks {
		names = append(names, d.Name)
	}
	want := []string{"parity", "disk1", "disk2", "disk3", "disk4", "boot", "cache", "fastcache", "flash"}
	if !slices.Equal(names, want) {
		t.Errorf("disks = %v, want %v without the empty slots", names, want)
	}
	if got := st.Devices(); !slices.Equal(got, []string{"cache", "fastcache", "disk1", "disk2", "disk3", "disk4"}) {
		t.Errorf("devices = %v", got)
	}

	disk1, _ := st.Disk("disk1")
	if disk1.Kind != "data" || !disk1.Spundown || disk1.Temp != nil || disk1.Free != 1664554016*1024 || disk1.Mount != "/mnt/disk1" {
		t.Errorf("disk1 = %+v", disk1)
	}
	disk2, _ := st.Disk("disk2")
	if disk2.Spundown || disk2.Temp == nil || *disk2.Temp != 35 {
		t.Errorf("disk2 = %+v", disk2)
	}
	if boot, _ := st.Disk("boot"); boot.Kind != "pool" || boot.Mounted {
		t.Errorf("an unmountable boot pool counts as mounted: %+v", boot)
	}
	if flash, _ := st.Disk("flash"); flash.Kind != "flash" || flash.Mount != "/boot" {
		t.Errorf("flash = %+v", flash)
	}
	if st.Mover != (Mover{Schedule: "0 */2 * * *"}) {
		t.Errorf("mover = %+v", st.Mover)
	}
}

func TestReadSharesFromUnraid734(t *testing.T) {
	st := readTestdata(t, testdataConfig())
	if len(st.Shares) != 22 {
		t.Fatalf("%d shares, want 21 from shares.ini and archiv from its cfg", len(st.Shares))
	}
	cases := []struct {
		name      string
		primary   Storage
		secondary *Storage
		mover     string
		disks     []string
	}{
		{"medien", Storage{"pool", "fastcache"}, &Storage{Type: "array"}, "toSecondary", []string{"disk1", "disk2", "disk3", "disk4"}},
		{"nextcloud", Storage{"pool", "cache"}, &Storage{Type: "array"}, "toSecondary", []string{"disk2", "disk3", "disk4"}},
		{"appdata", Storage{"pool", "cache"}, nil, "", []string{}},
		{"isos", Storage{Type: "array"}, nil, "", []string{"disk1", "disk2", "disk3", "disk4"}},
		{"archiv", Storage{"pool", "cache"}, &Storage{Type: "array"}, "toPrimary", []string{"disk2", "disk3"}},
	}
	for _, c := range cases {
		sh, ok := st.Share(c.name)
		if !ok {
			t.Errorf("%s is missing", c.name)
			continue
		}
		if sh.Primary != c.primary || (sh.Secondary == nil) != (c.secondary == nil) || sh.Secondary != nil && *sh.Secondary != *c.secondary {
			t.Errorf("%s: primary %+v secondary %+v", c.name, sh.Primary, sh.Secondary)
		}
		if sh.Mover != c.mover || !slices.Equal(sh.Disks, c.disks) {
			t.Errorf("%s: mover %q disks %v", c.name, sh.Mover, sh.Disks)
		}
	}
	medien, _ := st.Share("medien")
	if medien.Allocator != "highwater" || medien.Floor != 97628454*1024 || medien.Used != 43599798544*1024 {
		t.Errorf("medien = %+v", medien)
	}
	archiv, _ := st.Share("archiv")
	if archiv.Allocator != "mostfree" || archiv.SplitLevel != "1" || archiv.Floor != 50<<30 || archiv.Comment != "Altes Zeug" {
		t.Errorf("archiv = %+v", archiv)
	}
}

func TestParityFromMdstat(t *testing.T) {
	st := readTestdata(t, testdataConfig())
	p := st.Parity
	if p.Running || p.Busy() || p.Kind != "" || p.LastExit != -4 || p.LastSync == nil || p.LastSync.Unix() != 1793656441 {
		t.Errorf("idle parity = %+v", p)
	}

	c := testdataConfig()
	c.Mdstat = filepath.Join("testdata", "mdstat-check")
	p = readTestdata(t, c).Parity
	if !p.Running || p.Paused || !p.Busy() || p.Kind != "check" || p.Percent != 50 || p.Speed != 2000000*1024/10 {
		t.Errorf("running check = %+v", p)
	}
}

func TestParityPausedIsNotBusy(t *testing.T) {
	p := parityFrom(parseINI([]byte("mdResyncAction=recon D1\nmdResyncPos=100\nmdResync=0\nmdResyncSize=400\n"))[0])
	if !p.Running || !p.Paused || p.Busy() || p.Kind != "rebuild" || p.Percent != 25 {
		t.Errorf("paused rebuild = %+v", p)
	}
}

func TestMissingFilesMeanNotUnraid(t *testing.T) {
	c := DefaultConfig()
	dir := t.TempDir()
	c.Emhttp, c.Shares, c.Mdstat = dir, dir, filepath.Join(dir, "mdstat")
	st := readTestdata(t, c)
	if st.Available || len(st.Disks) != 0 || len(st.Shares) != 0 {
		t.Errorf("state = %+v", st)
	}
}

func TestPoolMembersFollowTheirPool(t *testing.T) {
	secs := parseINI([]byte("[\"cache\"]\nname=\"cache\"\ntype=\"Cache\"\nstatus=\"DISK_OK\"\nfsStatus=\"Mounted\"\n[\"cache2\"]\nname=\"cache2\"\ntype=\"Cache\"\nstatus=\"DISK_OK\"\nfsStatus=\"Mounted\"\n"))
	var disks []Disk
	for _, s := range secs {
		d, _ := diskFrom(s)
		disks = append(disks, d)
	}
	markPoolMembers(disks)
	if disks[0].Pool != "" || disks[1].Pool != "cache" || disks[1].Mounted {
		t.Errorf("disks = %+v", disks)
	}
}

func TestFloorBytes(t *testing.T) {
	cases := map[string]int64{"": 0, "0": 0, "1000": 1024000, "50GB": 50 << 30, "1.5T": 3 << 39, "200M": 200 << 20, "512K": 512 << 10, "junk": 0}
	for in, want := range cases {
		if got := floorBytes(in); got != want {
			t.Errorf("floorBytes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseINIKeepsSectionsAndUnquotes(t *testing.T) {
	secs := parseINI([]byte("# comment\n[\"a\"]\nx=\"1\"\ny=two words\n\n[\"b\"]\nx=\"\"\n"))
	if len(secs) != 2 || secs[0].name != "a" || secs[0].str("x") != "1" || secs[0].str("y") != "two words" || secs[1].str("x") != "" {
		t.Errorf("sections = %+v", secs)
	}
}

func TestExclusiveSharesCountOnlyWithTheirLinkToThePool(t *testing.T) {
	c := testdataConfig()
	c.Mnt = t.TempDir()
	for _, d := range []string{"user", "fastcache/code", "fastcache/featherdrop", "cache/elsewhere"} {
		if err := os.MkdirAll(filepath.Join(c.Mnt, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for link, target := range map[string]string{
		"code":        "../fastcache/code",
		"featherdrop": "../fastcache/featherdrop",
		"appdata":     "../cache/elsewhere",
		"backups":     filepath.Join(c.Mnt, "cache", "backups"),
	} {
		if err := os.Symlink(target, filepath.Join(c.Mnt, "user", link)); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	svc := New(c)
	for share, want := range map[string]string{"code": "fastcache", "backups": "cache", "featherdrop": "", "appdata": "", "domains": ""} {
		pool, ok := svc.ExclusivePool(share)
		if pool != want || ok != (want != "") {
			t.Errorf("ExclusivePool(%s) = %q, %v, want %q", share, pool, ok, want)
		}
	}
	if got := svc.UserShares(); got != filepath.Join(c.Mnt, "user") {
		t.Errorf("UserShares() = %s", got)
	}
}
