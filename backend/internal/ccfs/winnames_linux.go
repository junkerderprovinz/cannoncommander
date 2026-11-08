package ccfs

import (
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"golang.org/x/sys/unix"
)

// The magic numbers of the file systems Windows brings to a stick or a
// second disk, as statfs reports them for the kernel's own drivers.
const (
	exfatMagic = 0x2011BAB0
	ntfsMagic  = 0x5346544E
	ntfs3Magic = 0x7366746E
)

// WindowsNames reports whether names in dir follow the rules of Windows,
// as on a FAT, exFAT or NTFS stick in the desktop app, where a name
// Windows refuses would leave the file unreachable there. The container
// writes to the file systems of the server.
func WindowsNames(dir string) bool {
	if !desktop.Active() {
		return false
	}
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return false
	}
	switch uint32(st.Type) {
	case unix.MSDOS_SUPER_MAGIC, exfatMagic, ntfsMagic, ntfs3Magic:
		return true
	}
	return false
}
