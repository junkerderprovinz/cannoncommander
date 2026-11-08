//go:build !windows

package clipapi

import (
	"os"
	"syscall"
)

// sameVolume reports whether two paths lie on one file system, where a file
// manager moves what is dropped rather than copying it.
func sameVolume(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	if errA != nil || errB != nil {
		return false
	}
	sa, okA := ia.Sys().(*syscall.Stat_t)
	sb, okB := ib.Sys().(*syscall.Stat_t)
	return okA && okB && sa.Dev == sb.Dev
}
