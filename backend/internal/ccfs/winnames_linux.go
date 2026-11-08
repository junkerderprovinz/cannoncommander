package ccfs

import (
	"os"
	"strings"

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
	fstype := ""
	if uint32(st.Type) == unix.FUSE_SUPER_MAGIC {
		if info, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
			fstype = mountType(string(info), dir)
		}
	}
	return windowsFS(uint32(st.Type), fstype)
}

// windowsFS tells the file systems of Windows by the magic number of their
// driver. ntfs-3g and exfat-fuse report the one of FUSE, which sshfs and
// rclone share, and only the mount table names their type fuseblk, a FUSE
// file system on a block device of its own.
func windowsFS(magic uint32, fstype string) bool {
	switch magic {
	case unix.MSDOS_SUPER_MAGIC, exfatMagic, ntfsMagic, ntfs3Magic:
		return true
	case unix.FUSE_SUPER_MAGIC:
		return fstype == "fuseblk"
	}
	return false
}

// mountType returns the type of the file system that holds dir, from the
// mount table in mountinfo.
func mountType(mountinfo, dir string) string {
	at, fstype := "", ""
	for line := range strings.SplitSeq(mountinfo, "\n") {
		before, after, ok := strings.Cut(line, " - ")
		f, g := strings.Fields(before), strings.Fields(after)
		if !ok || len(f) < 5 || len(g) < 1 {
			continue
		}
		if m := unescapeMount(f[4]); within(dir, m) && len(m) >= len(at) {
			at, fstype = m, g[0]
		}
	}
	return fstype
}
