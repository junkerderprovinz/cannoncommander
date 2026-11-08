package ccfs

import (
	"io/fs"
	"syscall"
)

// attrsOf reads the attributes from what the folder listing brought along,
// so it costs no call of its own.
func attrsOf(d fs.DirEntry) uint32 {
	info, err := d.Info()
	if err != nil {
		return 0
	}
	if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return a.FileAttributes
	}
	return 0
}
