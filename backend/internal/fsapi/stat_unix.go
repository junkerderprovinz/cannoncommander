//go:build unix && !linux

package fsapi

import (
	"io/fs"
	"syscall"
)

func sysDetails(info fs.FileInfo, st *fileStat) {
	if s, ok := info.Sys().(*syscall.Stat_t); ok {
		st.ino, st.nlink = uint64(s.Ino), uint64(s.Nlink)
		st.ids, st.uid, st.gid = true, s.Uid, s.Gid
	}
}

func entryStat(real string, _ fs.DirEntry) (fileStat, error) { return lstatFile(real) }
