//go:build !windows

package fileops

import (
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func freeSpace(dir string) (int64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return int64(st.Bavail) * int64(st.Bsize), true
}

func device(info fs.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}

// Chown hands p to the owner of info. Without CAP_CHOWN the process may
// only keep its own ids, which is fine: the files then belong to the user
// the container runs as.
func Chown(p string, info fs.FileInfo) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		_ = os.Lchown(p, int(st.Uid), int(st.Gid))
	}
}
