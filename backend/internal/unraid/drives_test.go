package unraid

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// drivesEnv is a server with a ZFS mirror of two NVMe drives, two array
// disks and parity. disk1 sleeps, and so does the pool when poolAsleep is
// set.
func drivesEnv(t *testing.T, poolAsleep bool) (*env, *[]string) {
	t.Helper()
	e := newEnv(t)
	spun := map[bool]string{true: "1", false: "0"}
	disks := []map[string]string{
		{"name": "parity", "type": "Parity", "size": "4000000", "rotational": "1", "spundown": "0", "temp": "34", "transport": "ata"},
		{"name": "disk1", "type": "Data", "size": "4000000", "fsSize": "3900000", "fsFree": "100000", "fsUsed": "3800000", "fsType": "xfs", "fsStatus": "Mounted", "fsMountpoint": "/mnt/disk1", "rotational": "1", "spundown": "1", "temp": "*", "transport": "ata"},
		{"name": "disk2", "type": "Data", "size": "4000000", "fsSize": "3900000", "fsFree": "900000", "fsUsed": "3000000", "fsType": "xfs", "fsStatus": "Mounted", "fsMountpoint": "/mnt/disk2", "rotational": "1", "spundown": "0", "temp": "36", "transport": "ata"},
		{"name": "cache", "type": "Cache", "size": "1000000", "fsSize": "950000", "fsFree": "450000", "fsUsed": "500000", "fsType": "zfs", "fsProfile": "mirror", "fsStatus": "Mounted", "fsMountpoint": "/mnt/cache", "rotational": "0", "spundown": spun[poolAsleep], "temp": "41", "transport": "nvme", "device": "nvme0n1", "devices": "2"},
		{"name": "cache2", "type": "Cache", "size": "1000000", "fsStatus": "Mounted", "rotational": "0", "spundown": spun[poolAsleep], "temp": "43", "transport": "nvme", "device": "nvme1n1"},
		{"name": "flash", "type": "Flash", "size": "60000", "fsSize": "60000", "fsFree": "50000", "fsType": "vfat", "fsStatus": "Mounted", "fsMountpoint": "/boot"},
	}
	var b strings.Builder
	for _, d := range disks {
		fmt.Fprintf(&b, "[%q]\nstatus=\"DISK_OK\"\n", d["name"])
		for _, k := range slices.Sorted(func(yield func(string) bool) {
			for k := range d {
				if !yield(k) {
					return
				}
			}
		}) {
			fmt.Fprintf(&b, "%s=%q\n", k, d[k])
		}
	}
	e.write(filepath.Join(e.cfg.Emhttp, "disks.ini"), b.String())
	e.svc.fresh()
	var calls []string
	e.svc.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		switch {
		case name == "zpool" && args[0] == "get":
			return []byte("14\n"), nil
		case name == "zfs":
			return []byte("1.41x\n"), nil
		case name == "zpool" && args[0] == "status":
			return []byte("  pool: cache\n state: ONLINE\n  scan: scrub repaired 0B in 00:14:02 with 0 errors on Sun Nov  2 00:38:03 2026\nconfig:\n"), nil
		}
		return nil, fmt.Errorf("unexpected %s", name)
	}
	return e, &calls
}

func (e *env) drives() *Drives {
	e.t.Helper()
	d, err := e.svc.Drives(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

func driveNames(ds []Drive) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.Name)
	}
	return out
}

func TestDrivesListThePoolWithItsMembersThenTheArrayAndParity(t *testing.T) {
	e, _ := drivesEnv(t, false)
	d := e.drives()
	if got := driveNames(d.Pools); !slices.Equal(got, []string{"cache"}) {
		t.Fatalf("pools = %v", got)
	}
	pool := d.Pools[0]
	if got := driveNames(pool.Members); !slices.Equal(got, []string{"cache", "cache2"}) || pool.Members[1].Device != "nvme1n1" || *pool.Members[1].Temp != 43 {
		t.Errorf("members = %+v", pool.Members)
	}
	if pool.FsType != "zfs" || pool.Profile != "mirror" || pool.FsSize != 950000*1024 || pool.Used != 500000*1024 {
		t.Errorf("pool = %+v", pool)
	}
	if pool.Zfs == nil || pool.Zfs.Fragmentation == nil || *pool.Zfs.Fragmentation != 14 || pool.Zfs.CompressRatio != 1.41 || pool.Zfs.Scrub == nil || pool.Zfs.Scrub.Seconds != 14*60+2 {
		t.Errorf("zfs = %+v", pool.Zfs)
	}
	if got := driveNames(d.Array); !slices.Equal(got, []string{"disk1", "disk2", "parity"}) {
		t.Errorf("array = %v", got)
	}
	if !d.Array[0].Asleep || d.Array[1].Asleep || d.Array[2].Kind != "parity" {
		t.Errorf("array = %+v", d.Array)
	}
}

func TestDrivesNeverRunAToolForASleepingPool(t *testing.T) {
	e, calls := drivesEnv(t, false)
	e.svc.Observe(day0)
	e.drives()
	if len(*calls) != 3 {
		t.Fatalf("calls while awake = %v", *calls)
	}
	asleep, asleepCalls := drivesEnv(t, true)
	asleep.svc.history = e.svc.history
	pool := asleep.drives().Pools[0]
	if len(*asleepCalls) != 0 {
		t.Errorf("a sleeping pool was asked: %v", *asleepCalls)
	}
	if !pool.Asleep || pool.Zfs == nil || pool.Zfs.CompressRatio != 1.41 {
		t.Errorf("a sleeping pool shows %+v, want the facts from when it was awake", pool)
	}
}

func TestDrivesReadOnlyWhatTheServerAlreadyKnows(t *testing.T) {
	e, calls := drivesEnv(t, false)
	// Without any disk mounted the report still stands: it never looks at
	// a disk's file system, which would spin a sleeping one up.
	if err := os.RemoveAll(e.cfg.Mnt); err != nil {
		t.Fatal(err)
	}
	d := e.drives()
	if d.Array[0].FsSize != 3900000*1024 || d.Array[0].Used != 3800000*1024 {
		t.Errorf("disk1 = %+v", d.Array[0])
	}
	for _, c := range *calls {
		if !strings.HasPrefix(c, "zpool ") && !strings.HasPrefix(c, "zfs ") || !strings.HasSuffix(c, " cache") {
			t.Errorf("ran %q", c)
		}
	}
}

func TestDrivesShowTheLastKnownTemperatureOfASleepingDisk(t *testing.T) {
	e, _ := drivesEnv(t, false)
	h := NewHistory("")
	h.Observe(diskState(dataDisk("disk1", 1000, false, 33)), day0)
	h.Observe(diskState(dataDisk("disk1", 1000, true, 0)), day0.Add(time.Hour))
	e.svc.history = h
	disk1 := e.drives().Array[0]
	if disk1.Temp == nil || *disk1.Temp != 33 || disk1.TempAt == nil || !disk1.TempAt.Equal(day0) || disk1.AsleepSince == nil || !disk1.AsleepSince.Equal(day0.Add(time.Hour)) {
		t.Errorf("disk1 = %+v", disk1)
	}
	if disk2 := e.drives().Array[1]; disk2.TempAt != nil || *disk2.Temp != 36 {
		t.Errorf("disk2 = %+v", disk2)
	}
}

func TestDrivesCarryTheHistoryAndTheOutlook(t *testing.T) {
	e, _ := drivesEnv(t, false)
	h := NewHistory("")
	for i := range 28 {
		d := dataDisk("disk2", int64(3_000_000+i*10_000)*1024, false, 36)
		d.FsSize = 3_900_000 * 1024
		h.Observe(diskState(d), day0.AddDate(0, 0, i-27))
	}
	e.svc.history = h
	disk2 := e.drives().Array[1]
	if len(disk2.History) != 28 {
		t.Fatalf("history = %+v", disk2.History)
	}
	if w, ok := disk2.FullIn["4"]; !ok || w < 5 || w > 20 {
		t.Errorf("full in = %v", disk2.FullIn)
	}
}

func TestLastParityCheckComesFromTheLog(t *testing.T) {
	e, _ := drivesEnv(t, false)
	e.write(filepath.Join(e.cfg.BootConfig, "parity-checks.log"), "2026 Sep 12 03:10:00|50000|80.0 MB/s|0|0|check P|3900000|50000|1\n"+
		"2026 Oct 10 05:47:54|89155|44.9 MB/s|0|2|check P|3900000|89155|1\n"+
		"2026 Oct 20 01:00:00|600|1 MB/s|-4|0|check P|3900000|600|1\n")
	p := e.drives().Parity
	if p == nil || p.At.Format("2006-01-02 15:04") != "2026-10-20 01:00" || p.Exit != -4 {
		t.Fatalf("parity = %+v", p)
	}
	e.write(filepath.Join(e.cfg.BootConfig, "parity-checks.log"), "Oct 10 05:47:54 2026|89155|44.9 MB/s|0|2\n")
	if p := e.drives().Parity; p == nil || p.Errors != 2 || p.Seconds != 89155 || p.At.Day() != 10 {
		t.Errorf("older line = %+v", p)
	}
}

func TestLastParityCheckFallsBackToTheMdDriver(t *testing.T) {
	e, _ := drivesEnv(t, false)
	e.write(e.cfg.Mdstat, "mdState=STARTED\nmdResync=0\nmdResyncPos=0\nsbSynced=1793656441\nsbSynced2=1793696441\nsbSyncErrs=3\nsbSyncExit=0\n")
	e.svc.fresh()
	p := e.drives().Parity
	if p == nil || p.At.Unix() != 1793696441 || p.Errors != 3 || p.Seconds != 40000 {
		t.Errorf("parity = %+v", p)
	}
}

func TestScrubLineOfZpoolStatus(t *testing.T) {
	s, ok := parseScrub("  scan: scrub repaired 0B in 1 days 02:03:04 with 5 errors on Sun Nov  2 00:38:03 2026\n")
	if !ok || s.Errors != 5 || s.Seconds != 86400+2*3600+3*60+4 || s.At.Format("2006-01-02") != "2026-11-02" {
		t.Errorf("scrub = %+v, %v", s, ok)
	}
	if _, ok := parseScrub("  scan: none requested\n"); ok {
		t.Error("a pool never scrubbed has a scrub")
	}
}

func TestDrivesEndpoint(t *testing.T) {
	_, h := routesFixture(t)
	var d Drives
	if code := call(t, h, "anna", http.MethodGet, "/unraid/drives", "", &d); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(d.Array) == 0 {
		t.Errorf("drives = %+v", d)
	}
	if code := call(t, h, "", http.MethodGet, "/unraid/drives", "", nil); code != http.StatusUnauthorized {
		t.Errorf("without login: %d", code)
	}
}
