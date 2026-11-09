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

func bytesz(s string) dbus.Variant { return dbus.MakeVariant(append([]byte(s), 0)) }

// block is a Block interface the way udisks2 reports one.
func block(drive dbus.ObjectPath, label string, system, ignore bool, size uint64) map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Drive": dbus.MakeVariant(drive), "IdLabel": dbus.MakeVariant(label), "IdUsage": dbus.MakeVariant("filesystem"),
		"HintSystem": dbus.MakeVariant(system), "HintIgnore": dbus.MakeVariant(ignore), "Size": dbus.MakeVariant(size),
	}
}

func TestUdisksShowsWhatAFileManagerShows(t *testing.T) {
	const (
		disk  = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/Samsung_SSD")
		stick = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/SanDisk")
		dvd   = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/DVD")
	)
	objs := udisksObjects{
		disk:  {udisksDrive: {"Removable": dbus.MakeVariant(false), "ConnectionBus": dbus.MakeVariant("")}},
		stick: {udisksDrive: {"Removable": dbus.MakeVariant(true), "Ejectable": dbus.MakeVariant(false), "CanPowerOff": dbus.MakeVariant(true), "ConnectionBus": dbus.MakeVariant("usb")}},
		dvd:   {udisksDrive: {"Optical": dbus.MakeVariant(true), "Ejectable": dbus.MakeVariant(true)}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p1": {udisksBlock: block(disk, "", true, false, 1e9), udisksFS: {"MountPoints": mounts("/boot/efi")}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p2": {udisksBlock: block(disk, "", true, false, 4e11), udisksFS: {"MountPoints": mounts("/")}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p3": {udisksBlock: block(disk, "Daten", true, false, 16e9), udisksFS: {"MountPoints": mounts()}},
		"/org/freedesktop/UDisks2/block_devices/nvme0n1p4": {udisksBlock: block(disk, "Wiederherstellung", true, true, 1e9), udisksFS: {"MountPoints": mounts()}},
		"/org/freedesktop/UDisks2/block_devices/sdb1":      {udisksBlock: block(stick, "URLAUB", false, false, 16e9), udisksFS: {"MountPoints": mounts("/run/media/jdp/URLAUB")}},
		"/org/freedesktop/UDisks2/block_devices/sr0":       {udisksBlock: block(dvd, "FILM", false, false, 4e9), udisksFS: {"MountPoints": mounts()}},
		"/org/freedesktop/UDisks2/block_devices/sdc":       {udisksBlock: block(stick, "", false, false, 16e9)},
	}
	got := map[string]desktop.Volume{}
	for _, v := range udisksVolumes(objs, 1000) {
		got[filepath.Base(v.ID)] = v
	}
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	if v := got["nvme0n1p3"]; v.Root != "" || v.Kind != "fixed" || v.Label != "Daten" || v.Eject || v.Size != 16e9 {
		t.Errorf("an internal partition that waits to be mounted: %+v", v)
	}
	if v := got["sdb1"]; v.Root != "/run/media/jdp/URLAUB" || v.Kind != "removable" || !v.Eject {
		t.Errorf("a stick: %+v", v)
	}
	if v := got["sr0"]; v.Kind != "optical" || !v.Eject {
		t.Errorf("a disc: %+v", v)
	}
}

// The loop devices of an Ubuntu 24.04 desktop, as udisksctl dump lists them:
// every snap is a squashfs image root set up below /snap, and a disk image
// the person attached in GNOME Disks is set up by their own uid.
func TestSnapsAreNoDrivesButAnAttachedImageIs(t *testing.T) {
	loop := func(backing string, uid uint32, mount string, system bool) map[string]map[string]dbus.Variant {
		return map[string]map[string]dbus.Variant{
			udisksBlock: block("/", "", system, false, 74e6),
			udisksFS:    {"MountPoints": mounts(mount)},
			udisksLoop:  {"BackingFile": bytesz(backing), "SetupByUID": dbus.MakeVariant(uid), "Autoclear": dbus.MakeVariant(true)},
		}
	}
	objs := udisksObjects{
		"/org/freedesktop/UDisks2/block_devices/loop0": loop("/var/lib/snapd/snaps/bare_5.snap", 0, "/snap/bare/5", true),
		"/org/freedesktop/UDisks2/block_devices/loop1": loop("/var/lib/snapd/snaps/core24_609.snap", 0, "/snap/core24/609", true),
		"/org/freedesktop/UDisks2/block_devices/loop2": loop("/var/lib/snapd/snaps/firefox_5134.snap", 0, "/snap/firefox/5134", true),
		"/org/freedesktop/UDisks2/block_devices/loop3": loop("/var/lib/snapd/snaps/gnome-46-2404_77.snap", 0, "/snap/gnome-46-2404/77", true),
		"/org/freedesktop/UDisks2/block_devices/loop4": loop("/var/lib/snapd/snaps/snapd_23258.snap", 0, "/snap/snapd/23258", true),
		"/org/freedesktop/UDisks2/block_devices/loop5": loop("/var/lib/snapd/snaps/gtk-common-themes_1535.snap", 0, "/snap/gtk-common-themes/1535", true),
		"/org/freedesktop/UDisks2/block_devices/loop6": loop("/var/lib/snapd/snaps/snap-store_1216.snap", 0, "/snap/snap-store/1216", true),
		// A snap that is not mounted at the moment still is none of the person's.
		"/org/freedesktop/UDisks2/block_devices/loop7": loop("/var/lib/snapd/snaps/firefox_5091.snap", 0, "", false),
		"/org/freedesktop/UDisks2/block_devices/loop8": loop("/home/jdp/Downloads/ubuntu.iso", 1000, "/run/media/jdp/Ubuntu", false),
		"/org/freedesktop/UDisks2/block_devices/loop9": loop("/root/other.img", 0, "", false),
	}
	got := udisksVolumes(objs, 1000)
	if len(got) != 1 || filepath.Base(got[0].ID) != "loop8" || got[0].Kind != "virtual" || !got[0].Eject || got[0].Root != "/run/media/jdp/Ubuntu" {
		t.Fatalf("got %+v", got)
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
