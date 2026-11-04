package fsapi

import (
	"io/fs"

	"golang.org/x/sys/windows"
)

// ownerOf has nothing to report on Windows, which only runs in development.
func ownerOf(fs.FileInfo) (owner, group string) { return "", "" }

func usage(dir string) (free, total uint64, ok bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0, false
	}
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, nil); err != nil {
		return 0, 0, false
	}
	return free, total, true
}
