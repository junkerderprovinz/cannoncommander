package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

func mounts(paths ...string) dbus.Variant {
	var out [][]byte
	for _, p := range paths {
		out = append(out, append([]byte(p), 0))
	}
	return dbus.MakeVariant(out)
}

func TestUdisksShowsSticksAndDiscsButNotTheSystem(t *testing.T) {
	const (
		disk  = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/Samsung_SSD")
		stick = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/SanDisk")
		dvd   = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/DVD")
	)
	block := func(drive dbus.ObjectPath, label string, system bool) map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"Drive": dbus.MakeVariant(drive), "IdLabel": dbus.MakeVariant(label),
			"HintSystem": dbus.MakeVariant(system), "HintIgnore": dbus.MakeVariant(false), "Size": dbus.MakeVariant(uint64(16e9)),
		}
	}
	objs := udisksObjects{
		disk:  {udisksDrive: {"Removable": dbus.MakeVariant(false), "ConnectionBus": dbus.MakeVariant("")}},
		stick: {udisksDrive: {"Removable": dbus.MakeVariant(true), "Ejectable": dbus.MakeVariant(false), "CanPowerOff": dbus.MakeVariant(true), "ConnectionBus": dbus.MakeVariant("usb")}},
		dvd:   {udisksDrive: {"Optical": dbus.MakeVariant(true), "Ejectable": dbus.MakeVariant(true)}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p2": {udisksBlock: block(disk, "", true), udisksFS: {"MountPoints": mounts("/")}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p3": {udisksBlock: block(disk, "Daten", false), udisksFS: {"MountPoints": mounts()}},
		"/org/freedesktop/UDisks2/block_devices/sdb1":      {udisksBlock: block(stick, "URLAUB", true), udisksFS: {"MountPoints": mounts("/run/media/jdp/URLAUB")}},
		"/org/freedesktop/UDisks2/block_devices/sr0":       {udisksBlock: block(dvd, "FILM", false), udisksFS: {"MountPoints": mounts()}},
		"/org/freedesktop/UDisks2/block_devices/loop0":     {udisksBlock: block("/", "", false), udisksFS: {"MountPoints": mounts("/run/media/jdp/disk")}, udisksLoop: {}},
		"/org/freedesktop/UDisks2/block_devices/sdc":       {udisksBlock: block(stick, "", false)},
	}
	got := map[string]desktop.Volume{}
	for _, v := range udisksVolumes(objs) {
		got[filepath.Base(v.ID)] = v
	}
	if len(got) != 4 {
		t.Fatalf("got %+v", got)
	}
	if v := got["nvme0n1p3"]; v.Root != "" || v.Kind != "fixed" || v.Label != "Daten" || v.Eject || v.Size != 16e9 {
		t.Errorf("an unmounted partition: %+v", v)
	}
	if v := got["sdb1"]; v.Root != "/run/media/jdp/URLAUB" || v.Kind != "removable" || !v.Eject {
		t.Errorf("a stick: %+v", v)
	}
	if v := got["sr0"]; v.Kind != "optical" || !v.Eject {
		t.Errorf("a disc: %+v", v)
	}
	if v := got["loop0"]; v.Kind != "virtual" || !v.Eject {
		t.Errorf("a disk image: %+v", v)
	}
}

func TestGvfsFoldersReadAsTheAddressTheyStandFor(t *testing.T) {
	cases := map[string][2]string{
		"smb-share:server=nas,share=daten":  {"daten", "smb://nas/daten"},
		"sftp:host=pi.local,user=jdp":       {"pi.local", "sftp://jdp@pi.local"},
		"dav:host=cloud.example,ssl=true":   {"cloud.example", "davs://cloud.example"},
		"afp-volume:host=mac,volume=Archiv": {"Archiv", "afp://mac/Archiv"},
	}
	for folder, want := range cases {
		if label, remote := gvfsName(folder); label != want[0] || remote != want[1] {
			t.Errorf("%s: %s %s", folder, label, remote)
		}
	}
}

func TestEveryShareGvfsMountedIsANetworkVolume(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "smb-share:server=nas,share=daten"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := gvfsVolumes(dir)
	root := filepath.Join(dir, "smb-share:server=nas,share=daten")
	if len(got) != 1 || got[0].Root != root || got[0].ID != gvfsID+root || got[0].Kind != "network" || got[0].Remote != "smb://nas/daten" {
		t.Fatalf("got %+v", got)
	}
	if gvfsVolumes(filepath.Join(dir, "nothing")) != nil {
		t.Fatal("a missing gvfs folder has shares")
	}
}

func TestTheOldDefaultMakesWayForTheRoot(t *testing.T) {
	old := []storedSource{
		{Name: "jdp", Path: "/home/jdp", Indexed: true, SkipHidden: true, Exclude: []string{}},
		{Name: "media", Path: "/media", Exclude: []string{}},
		{Name: "mnt", Path: "/mnt"},
	}
	if !wroteBefore(old, "/home/jdp") {
		t.Fatal("the old default stayed")
	}
	changed := append([]storedSource(nil), old...)
	changed[2].ReadOnly = true
	if wroteBefore(changed, "/home/jdp") {
		t.Fatal("a source someone changed was replaced")
	}
	added := append(append([]storedSource(nil), old...), storedSource{Name: "data", Path: "/data"})
	if wroteBefore(added, "/home/jdp") {
		t.Fatal("a source someone added was replaced")
	}
	if wroteBefore([]storedSource{{Name: "Computer", Path: "/", Indexed: true, SkipHidden: true}}, "/home/jdp") {
		t.Fatal("today's default counts as old")
	}
}

func TestLinuxStartsWithTheRootAndSearchesTheHome(t *testing.T) {
	got := defaultSources("/home/jdp")
	if len(got) != 1 || got[0].Path != "/" || !got[0].Indexed || got[0].Include != "/home/jdp" {
		t.Fatalf("got %+v", got)
	}
}
