package fsapi

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// userShares is where Unraid joins its disks and pools into the user shares.
const userShares = "/mnt/user"

// Everything next to /mnt/user that is not one of these is a disk or a pool.
var (
	notDevices = map[string]bool{"user": true, "user0": true, "disks": true, "remotes": true, "addons": true, "rootshare": true}
	diskName   = regexp.MustCompile(`^disk[0-9]+$`)
)

const devicesTTL = time.Minute

var deviceCache struct {
	sync.Mutex
	root  string
	at    time.Time
	names []string
}

// devicesNextTo returns the disks and pools beside the user shares. A share
// keeps its files on them at the same path inside.
func devicesNextTo(userRoot string) []string {
	deviceCache.Lock()
	defer deviceCache.Unlock()
	if deviceCache.root == userRoot && time.Since(deviceCache.at) < devicesTTL {
		return deviceCache.names
	}
	var names []string
	dirents, _ := os.ReadDir(filepath.Dir(userRoot))
	for _, d := range dirents {
		if d.IsDir() && !notDevices[d.Name()] {
			names = append(names, d.Name())
		}
	}
	deviceCache.root, deviceCache.at, deviceCache.names = userRoot, time.Now(), names
	return names
}

func isUserShare(loc ccfs.Location) bool {
	return loc.Source != nil && loc.Source.Mount == userShares
}

// unraidDetails sets Pool on the entries of a user share folder that sit on
// a pool, which takes one folder read per pool. It also drops the inode
// numbers, because the share makes up its own and only the disk's count.
func unraidDetails(dir ccfs.Location, items []Item) {
	if !isUserShare(dir) || len(items) == 0 {
		return
	}
	for i := range items {
		items[i].Ino = 0
	}
	parent := filepath.Dir(dir.Source.Root)
	for _, dev := range devicesNextTo(dir.Source.Root) {
		if diskName.MatchString(dev) {
			continue
		}
		dirents, err := os.ReadDir(filepath.Join(parent, dev, filepath.FromSlash(dir.Index)))
		if err != nil {
			continue
		}
		on := make(map[string]bool, len(dirents))
		for _, d := range dirents {
			on[d.Name()] = true
		}
		for i := range items {
			if items[i].Pool == "" && on[items[i].Name] {
				items[i].Pool = dev
			}
		}
	}
}

// devicesOf finds the disks and pools that hold a path of the user shares,
// with one stat each, and takes the inode from the first.
func devicesOf(loc ccfs.Location, item *Item) []string {
	if !isUserShare(loc) {
		return nil
	}
	item.Ino = 0
	parent := filepath.Dir(loc.Source.Root)
	var out []string
	for _, dev := range devicesNextTo(loc.Source.Root) {
		st, err := lstatFile(filepath.Join(parent, dev, filepath.FromSlash(loc.Index)))
		if err != nil {
			continue
		}
		if len(out) == 0 {
			item.Ino = st.ino
		}
		if item.Pool == "" && !diskName.MatchString(dev) {
			item.Pool = dev
		}
		out = append(out, dev)
	}
	return out
}
