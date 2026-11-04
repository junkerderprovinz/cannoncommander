//go:build !windows

package search

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

type statInfo struct {
	mode       uint32
	uid, gid   uint32
	nlink, ino uint64
	link       bool
	target     string
	owner      string
	group      string
}

var (
	namesMu    sync.Mutex
	ownerNames = map[uint32]string{}
	groupNames = map[uint32]string{}
)

// nameOf looks a user or group number up once and keeps the name, the
// search asks for the same few owners over and over.
func nameOf(cache map[uint32]string, id uint32, lookup func(string) (string, error)) string {
	namesMu.Lock()
	defer namesMu.Unlock()
	if n, ok := cache[id]; ok {
		return n
	}
	n, err := lookup(strconv.FormatUint(uint64(id), 10))
	if err != nil {
		n = strconv.FormatUint(uint64(id), 10)
	}
	cache[id] = n
	return n
}

func statOf(real string) statInfo {
	info, err := os.Lstat(real)
	if err != nil {
		return statInfo{}
	}
	var s statInfo
	if info.Mode()&fs.ModeSymlink != 0 {
		s.link = true
		if t, err := os.Readlink(real); err == nil {
			s.target = filepath.ToSlash(t)
		}
	}
	s.mode = uint32(info.Mode().Perm())
	if info.Mode()&fs.ModeSetuid != 0 {
		s.mode |= 0o4000
	}
	if info.Mode()&fs.ModeSetgid != 0 {
		s.mode |= 0o2000
	}
	if info.Mode()&fs.ModeSticky != 0 {
		s.mode |= 0o1000
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		s.uid, s.gid = st.Uid, st.Gid
		s.nlink, s.ino = uint64(st.Nlink), uint64(st.Ino)
		s.owner = nameOf(ownerNames, s.uid, func(id string) (string, error) {
			u, err := user.LookupId(id)
			if err != nil {
				return "", err
			}
			return u.Username, nil
		})
		s.group = nameOf(groupNames, s.gid, func(id string) (string, error) {
			g, err := user.LookupGroupId(id)
			if err != nil {
				return "", err
			}
			return g.Name, nil
		})
	}
	return s
}
