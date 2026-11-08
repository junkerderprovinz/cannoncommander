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

// entryStat takes what reading the folder brought along for a file, which
// saves a second call per entry. NTFS updates the entry of a folder in its
// parent late, so a folder is asked itself.
func entryStat(real string, d fs.DirEntry) (fileStat, error) {
	if d.IsDir() {
		return lstatFile(real)
	}
	info, err := d.Info()
	if err != nil {
		return fileStat{}, err
	}
	return fromInfo(info), nil
}
