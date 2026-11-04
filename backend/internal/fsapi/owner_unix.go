//go:build unix

package fsapi

import (
	"os/user"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	idNamesMu sync.Mutex
	idNames   = map[string]string{}
)

// ownerNames returns user and group names, or the numbers when the
// container has no entry for them in its passwd and group files.
func ownerNames(st fileStat) (owner, group string) {
	if !st.ids {
		return "", ""
	}
	return lookupName("u", st.uid), lookupName("g", st.gid)
}

func lookupName(kind string, id uint32) string {
	num := strconv.FormatUint(uint64(id), 10)
	key := kind + num
	idNamesMu.Lock()
	defer idNamesMu.Unlock()
	if n, ok := idNames[key]; ok {
		return n
	}
	n := num
	if kind == "u" {
		if u, err := user.LookupId(num); err == nil {
			n = u.Username
		}
	} else if g, err := user.LookupGroupId(num); err == nil {
		n = g.Name
	}
	idNames[key] = n
	return n
}

// usage returns free and total bytes of the file system holding dir.
func usage(dir string) (free, total uint64, ok bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, 0, false
	}
	bs := uint64(st.Bsize)
	return uint64(st.Bavail) * bs, uint64(st.Blocks) * bs, true
}
