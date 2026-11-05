//go:build !windows

package ccfs

import (
	"io/fs"
	"syscall"
)

const noFollow = syscall.O_NOFOLLOW

func singleLink(info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return !ok || st.Nlink == 1
}
