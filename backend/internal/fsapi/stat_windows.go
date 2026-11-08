package fsapi

import (
	"io/fs"
	"syscall"
	"time"
)

func sysDetails(info fs.FileInfo, st *fileStat) {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		st.btime = time.Unix(0, d.CreationTime.Nanoseconds())
		st.attrs = d.FileAttributes
	}
}
