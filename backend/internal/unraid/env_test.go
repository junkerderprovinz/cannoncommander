package unraid

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// env is a small Unraid on disk: three array disks and a cache pool under
// mnt, with emhttp files that describe them. Sizes in the ini files are
// KiB, as emhttp writes them.
type env struct {
	t   *testing.T
	svc *Service
	cfg Config
}

type fakeDisk struct {
	name, kind       string
	sizeKiB, freeKiB int64
	asleep           bool
}

var fakeDisks = []fakeDisk{
	{"cache", "Cache", 2048, 200, false},
	{"disk1", "Data", 4096, 48, true},
	{"disk2", "Data", 4096, 900, false},
	{"disk3", "Data", 4096, 1100, false},
}

// Each share is name, useCache, cachePool, allocator, floor in KiB,
// exclude and split level.
var fakeShares = [][7]string{
	{"medien", "no", "", "highwater", "50", "", ""},
	{"filme", "no", "", "fillup", "30", "", ""},
	{"backups", "no", "", "mostfree", "20", "disk1", ""},
	{"voll", "no", "", "highwater", "2000", "", ""},
	{"downloads", "yes", "cache", "highwater", "10", "", ""},
	{"appdata", "only", "cache", "highwater", "0", "", ""},
	{"serien", "no", "", "mostfree", "0", "", "1"},
}

func newEnv(t *testing.T) *env {
	t.Helper()
	base := t.TempDir()
	cfg := DefaultConfig()
	cfg.Emhttp = filepath.Join(base, "emhttp")
	cfg.Shares = filepath.Join(base, "shares")
	cfg.BootConfig = filepath.Join(base, "config")
	cfg.Mdstat = filepath.Join(base, "mdstat")
	cfg.Mnt = filepath.Join(base, "mnt")
	cfg.Notifications = filepath.Join(base, "notifications")
	cfg.Socket = filepath.Join(base, "emhttpd.socket")
	cfg.Poll = 20 * time.Millisecond
	e := &env{t: t, cfg: cfg}
	e.writeVar("")
	e.writeDisks(nil)

	var shares strings.Builder
	for _, s := range fakeShares {
		fmt.Fprintf(&shares, "[\"%s\"]\nname=\"%s\"\nuseCache=\"%s\"\ncachePool=\"%s\"\nallocator=\"%s\"\nfloor=\"%s\"\nexclude=\"%s\"\nsplitLevel=\"%s\"\nused=\"7\"\n",
			s[0], s[0], s[1], s[2], s[3], s[4], s[5], s[6])
	}
	e.write(filepath.Join(cfg.Emhttp, "shares.ini"), shares.String())
	e.write(filepath.Join(cfg.Mdstat), "mdState=STARTED\nmdResync=0\nmdResyncPos=0\n")
	for _, vol := range []string{"user", "disk1", "disk2", "disk3", "cache"} {
		if err := os.MkdirAll(filepath.Join(cfg.Mnt, vol), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e.svc = New(cfg)
	return e
}

func (e *env) write(p, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) writeVar(extra string) {
	e.write(filepath.Join(e.cfg.Emhttp, "var.ini"), "version=\"7.3.4\"\nNAME=\"Tower\"\nmdState=\"STARTED\"\ncsrf_token=\"T0KEN\"\n"+extra)
}

// writeDisks writes disks.ini, with the disks in awake spun up.
func (e *env) writeDisks(awake map[string]bool) {
	var b strings.Builder
	for _, d := range fakeDisks {
		spun := "0"
		if d.asleep && !awake[d.name] {
			spun = "1"
		}
		fmt.Fprintf(&b, "[\"%s\"]\nname=\"%s\"\ntype=\"%s\"\nstatus=\"DISK_OK\"\nfsStatus=\"Mounted\"\nfsMountpoint=\"/mnt/%s\"\nsize=\"%d\"\nfsSize=\"%d\"\nfsFree=\"%d\"\nspundown=\"%s\"\n",
			d.name, d.name, d.kind, d.name, d.sizeKiB, d.sizeKiB, d.freeKiB, spun)
	}
	e.write(filepath.Join(e.cfg.Emhttp, "disks.ini"), b.String())
	if e.svc != nil {
		e.svc.fresh()
	}
}

// file creates a sparse file of size KiB on a disk and returns its user
// share path.
func (e *env) file(dev, rel string, kib int64) string {
	e.t.Helper()
	p := filepath.Join(e.cfg.Mnt, dev, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := f.Truncate(kib * 1024); err != nil {
		e.t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		e.t.Fatal(err)
	}
	return "/mnt/user/" + rel
}

func (e *env) dir(dev, rel string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Join(e.cfg.Mnt, dev, filepath.FromSlash(rel)), 0o755); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) state() *State {
	e.t.Helper()
	st, err := e.svc.State()
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}
