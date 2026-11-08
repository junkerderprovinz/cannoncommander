//go:build unix

package plainpath

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// Link reports whether fi, from Lstat, is a link.
func Link(fi fs.FileInfo) bool { return fi.Mode()&fs.ModeSymlink != 0 }

// Mine reports whether p itself, not what it may point to, belongs to the
// user running the program.
func Mine(p string) (bool, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return false, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, errors.New("no owner")
	}
	return int(st.Uid) == os.Getuid(), nil
}
