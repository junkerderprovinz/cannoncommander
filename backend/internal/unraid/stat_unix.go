//go:build unix

package unraid

import (
	"io/fs"
	"syscall"
)

func inodeOf(info fs.FileInfo) inode {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return inode{}
	}
	return inode{dev: uint64(st.Dev), ino: st.Ino, nlink: uint64(st.Nlink)}
}

func ownerOf(info fs.FileInfo) (uid, gid int, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}
