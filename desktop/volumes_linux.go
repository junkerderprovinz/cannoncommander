package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

const (
	udisks        = "org.freedesktop.UDisks2"
	udisksPath    = "/org/freedesktop/UDisks2"
	udisksBlock   = udisks + ".Block"
	udisksFS      = udisks + ".Filesystem"
	udisksDrive   = udisks + ".Drive"
	udisksLoop    = udisks + ".Loop"
	gvfsPollEvery = 2 * time.Second

	settleAfterSignal = 300 * time.Millisecond
)

// interactive lets polkit ask for a password where mounting or ejecting
// needs rights, in the dialog of the desktop, as its file manager does.
var interactive = map[string]dbus.Variant{"auth.no_user_interaction": dbus.MakeVariant(false)}

// askAllowed marks a call that may wait while polkit asks the person.
const askAllowed = dbus.FlagAllowInteractiveAuthorization

// volumeWatch follows udisks2, which reports every stick, disc and
// partition, and the folder where gvfs mounts the shares the desktop
// opened, such as smb://nas/daten.
type volumeWatch struct {
	once    sync.Once
	mu      sync.Mutex
	conn    *dbus.Conn
	changed func()
	later   *time.Timer
}

// signal tells the server once a burst of signals about one device is over:
// udisks2 reports a stick's drive, its partitions and their mounts one by
// one.
func (w *volumeWatch) signal() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.later != nil {
		w.later.Stop()
	}
	w.later = time.AfterFunc(settleAfterSignal, w.changed)
}

// bus is the connection to udisks2, nil on a system without it.
func (w *volumeWatch) bus() *dbus.Conn {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn == nil {
		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			return nil
		}
		w.conn = conn
	}
	return w.conn
}

type udisksObjects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

func (w *volumeWatch) objects() udisksObjects {
	conn := w.bus()
	if conn == nil {
		return nil
	}
	var objs udisksObjects
	if err := conn.Object(udisks, udisksPath).Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs); err != nil {
		return nil
	}
	return objs
}

func (s *shell) Volumes() []desktop.Volume {
	return append(udisksVolumes(s.volumes.objects(), uint32(os.Getuid())), gvfsVolumes(gvfsDir())...)
}

func (s *shell) OnVolumes(changed func()) {
	w := &s.volumes
	w.mu.Lock()
	w.changed = changed
	w.mu.Unlock()
	w.once.Do(func() {
		if conn := w.bus(); conn != nil {
			go w.listen(conn)
		}
		go pollGvfs(gvfsDir(), w.signal)
	})
}

// listen tells the server of every signal udisks2 sends: a device that
// comes or goes and a file system mounted or unmounted.
func (w *volumeWatch) listen(conn *dbus.Conn) {
	if err := conn.AddMatchSignal(dbus.WithMatchSender(udisks)); err != nil {
		log.Printf("drives: udisks2 does not report its changes: %v", err)
		return
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	for range signals {
		w.signal()
	}
}

// udisksVolumes picks the file systems a file manager shows to the person
// with uid, as Nautilus does: a partition that waits to be mounted, and one
// mounted where the desktop puts the disks a person opens. Whatever udisks2
// says to ignore stays out, and so does a file system mounted anywhere else,
// such as / and /boot/efi. Of the loop devices only a disk image the person
// attached counts, never one of the snaps root sets up from /var/lib/snapd.
func udisksVolumes(objs udisksObjects, uid uint32) []desktop.Volume {
	var out []desktop.Volume
	for p, ifaces := range objs {
		block, fs := ifaces[udisksBlock], ifaces[udisksFS]
		if block == nil || fs == nil || boolProp(block, "HintIgnore") {
			continue
		}
		v := desktop.Volume{ID: string(p), Root: firstMount(fs["MountPoints"]), Label: stringProp(block, "IdLabel"), Kind: "fixed"}
		if v.Root != "" && !userMount(v.Root) {
			continue
		}
		if size, ok := block["Size"].Value().(uint64); ok {
			v.Size = size
		}
		drive := objs[pathProp(block, "Drive")][udisksDrive]
		if loop := ifaces[udisksLoop]; loop != nil {
			setupBy, _ := loop["SetupByUID"].Value().(uint32)
			if setupBy != uid || strings.HasPrefix(bytesProp(loop, "BackingFile"), "/var/lib/snapd/") {
				continue
			}
			v.Kind, v.Eject = "virtual", true
		} else if drive != nil && boolProp(drive, "Optical") {
			v.Kind, v.Eject = "optical", true
		} else if drive != nil && (boolProp(drive, "Removable") || slices.Contains([]string{"usb", "sdio", "ieee1394"}, stringProp(drive, "ConnectionBus"))) {
			v.Kind, v.Eject = "removable", boolProp(drive, "Ejectable") || boolProp(drive, "CanPowerOff")
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b desktop.Volume) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// userMount reports whether a file system is mounted where the desktop
// puts the disks a person opens.
func userMount(root string) bool {
	for _, base := range []string{"/media/", "/run/media/", "/mnt/"} {
		if strings.HasPrefix(root, base) {
			return true
		}
	}
	return false
}

func boolProp(props map[string]dbus.Variant, name string) bool {
	b, _ := props[name].Value().(bool)
	return b
}

func stringProp(props map[string]dbus.Variant, name string) string {
	s, _ := props[name].Value().(string)
	return s
}

func pathProp(props map[string]dbus.Variant, name string) dbus.ObjectPath {
	p, _ := props[name].Value().(dbus.ObjectPath)
	return p
}

// bytesProp reads a path udisks2 keeps as a byte string that ends in a NUL.
func bytesProp(props map[string]dbus.Variant, name string) string {
	b, _ := props[name].Value().([]byte)
	return strings.TrimRight(string(b), "\x00")
}

// firstMount reads the first of the mount points udisks2 lists as byte
// strings that end in a NUL.
func firstMount(v dbus.Variant) string {
	mounts, _ := v.Value().([][]byte)
	if len(mounts) == 0 {
		return ""
	}
	return strings.TrimRight(string(mounts[0]), "\x00")
}

func (s *shell) Mount(id string) (string, error) {
	conn := s.volumes.bus()
	if conn == nil {
		return "", errors.New("udisks2 is not there to mount it")
	}
	var root string
	err := conn.Object(udisks, dbus.ObjectPath(id)).Call(udisksFS+".Mount", askAllowed, interactive).Store(&root)
	return root, err
}

// Eject unmounts every file system on the volume's drive and lets the drive
// go: a disc comes out, a stick powers down and a disk image is detached.
// A share gvfs opened is unmounted.
func (s *shell) Eject(id string) error {
	if root, ok := strings.CutPrefix(id, gvfsID); ok {
		return exec.Command("gio", "mount", "--unmount", root).Run()
	}
	conn := s.volumes.bus()
	objs := s.volumes.objects()
	ifaces := objs[dbus.ObjectPath(id)]
	if conn == nil || ifaces == nil {
		return errors.New("udisks2 does not know this volume")
	}
	drive := pathProp(ifaces[udisksBlock], "Drive")
	for p, other := range objs {
		if p != dbus.ObjectPath(id) && (drive == "/" || pathProp(other[udisksBlock], "Drive") != drive) {
			continue
		}
		if fs := other[udisksFS]; fs != nil && firstMount(fs["MountPoints"]) != "" {
			if err := conn.Object(udisks, p).Call(udisksFS+".Unmount", askAllowed, interactive).Err; err != nil {
				return fmt.Errorf("unmount %s: %w", p, err)
			}
		}
	}
	if ifaces[udisksLoop] != nil {
		return conn.Object(udisks, dbus.ObjectPath(id)).Call(udisksLoop+".Delete", askAllowed, interactive).Err
	}
	props := objs[drive][udisksDrive]
	switch {
	case boolProp(props, "Ejectable"):
		return conn.Object(udisks, drive).Call(udisksDrive+".Eject", askAllowed, interactive).Err
	case boolProp(props, "CanPowerOff"):
		return conn.Object(udisks, drive).Call(udisksDrive+".PowerOff", askAllowed, interactive).Err
	}
	return nil
}

// gvfsID starts the id of a share that gvfs mounted, followed by its folder.
const gvfsID = "gvfs:"

// gvfsDir is where gvfs mounts the shares the desktop opened.
func gvfsDir() string {
	return filepath.Join("/run/user", fmt.Sprint(os.Getuid()), "gvfs")
}

// gvfsVolumes lists the shares in dir, each named the way gvfs names its
// folders, as smb-share:server=nas,share=daten.
func gvfsVolumes(dir string) []desktop.Volume {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []desktop.Volume
	for _, e := range entries {
		root := filepath.Join(dir, e.Name())
		label, remote := gvfsName(e.Name())
		out = append(out, desktop.Volume{ID: gvfsID + root, Root: root, Label: label, Kind: "network", Remote: remote, Eject: true})
	}
	return out
}

// gvfsName turns the folder gvfs mounts a share at into a name and an
// address: smb-share:server=nas,share=daten is daten at smb://nas/daten.
func gvfsName(folder string) (label, remote string) {
	kind, rest, _ := strings.Cut(folder, ":")
	keys := map[string]string{}
	for pair := range strings.SplitSeq(rest, ",") {
		k, v, _ := strings.Cut(pair, "=")
		keys[k] = v
	}
	scheme, _, _ := strings.Cut(kind, "-")
	if scheme == "dav" && keys["ssl"] == "true" {
		scheme = "davs"
	}
	host := keys["host"]
	if host == "" {
		host = keys["server"]
	}
	if u := keys["user"]; u != "" {
		host = u + "@" + host
	}
	share := keys["share"]
	if share == "" {
		share = keys["volume"]
	}
	remote = scheme + "://" + host
	if share != "" {
		remote += "/" + share
	}
	label = share
	if label == "" {
		label = keys["host"] + keys["server"]
	}
	return label, remote
}

// pollGvfs signals whenever the shares in dir change.
func pollGvfs(dir string, signal func()) {
	names := func() []string {
		entries, _ := os.ReadDir(dir)
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Name()
		}
		return out
	}
	last := names()
	for range time.Tick(gvfsPollEvery) {
		if now := names(); !slices.Equal(now, last) {
			last = now
			signal()
		}
	}
}
