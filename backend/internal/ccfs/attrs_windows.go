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
	return infoAttrs(info)
}

func infoAttrs(info fs.FileInfo) uint32 {
	if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return a.FileAttributes
	}
	return 0
}

// InCloud reports whether reading the file at p would download it from the
// cloud first, as with a file that OneDrive keeps online only.
func InCloud(p string) bool {
	name, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(name)
	return err == nil && CloudOnly(attrs)
}
