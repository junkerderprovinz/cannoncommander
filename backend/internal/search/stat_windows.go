package search

import (
	"io/fs"
	"os"
	"path/filepath"
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

// Windows has no owners by number, the fields stay empty there.
func statOf(real string) statInfo {
	info, err := os.Lstat(real)
	if err != nil {
		return statInfo{}
	}
	s := statInfo{mode: uint32(info.Mode().Perm()), nlink: 1}
	if info.Mode()&fs.ModeSymlink != 0 {
		s.link = true
		if t, err := os.Readlink(real); err == nil {
			s.target = filepath.ToSlash(t)
		}
	}
	return s
}
