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

// Everything else next to /mnt/user that is not an array disk is a pool.
var (
	notPools = map[string]bool{"user": true, "user0": true, "disks": true, "remotes": true, "addons": true, "rootshare": true}
	diskName = regexp.MustCompile(`^disk[0-9]+$`)
)

const poolsTTL = time.Minute

var poolCache struct {
	sync.Mutex
	root  string
	at    time.Time
	names []string
}

// poolsNextTo returns the pools beside the user shares, such as cache. A
// share keeps its files on the pools and the disks at the same path inside,
// so the pool folder shows which entries sit on fast storage.
func poolsNextTo(userRoot string) []string {
	poolCache.Lock()
	defer poolCache.Unlock()
	if poolCache.root == userRoot && time.Since(poolCache.at) < poolsTTL {
		return poolCache.names
	}
	var names []string
	dirents, _ := os.ReadDir(filepath.Dir(userRoot))
	for _, d := range dirents {
		if d.IsDir() && !notPools[d.Name()] && !diskName.MatchString(d.Name()) {
			names = append(names, d.Name())
		}
	}
	poolCache.root, poolCache.at, poolCache.names = userRoot, time.Now(), names
	return names
}

// markPools sets Pool on the entries of a user share folder that are on a
// pool. One folder read per pool covers the whole listing.
func markPools(dir ccfs.Location, items []Item) {
	if dir.Source == nil || dir.Source.Mount != userShares || len(items) == 0 {
		return
	}
	parent := filepath.Dir(dir.Source.Root)
	for _, pool := range poolsNextTo(dir.Source.Root) {
		dirents, err := os.ReadDir(filepath.Join(parent, pool, filepath.FromSlash(dir.Index)))
		if err != nil {
			continue
		}
		on := make(map[string]bool, len(dirents))
		for _, d := range dirents {
			on[d.Name()] = true
		}
		for i := range items {
			if items[i].Pool == "" && on[items[i].Name] {
				items[i].Pool = pool
			}
		}
	}
}
