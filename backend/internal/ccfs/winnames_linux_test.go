package ccfs

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTheFileSystemsOfWindowsAreKnownByTheirMagic(t *testing.T) {
	cases := []struct {
		magic  uint32
		fstype string
		want   bool
	}{
		{unix.MSDOS_SUPER_MAGIC, "vfat", true},
		{exfatMagic, "exfat", true},
		{ntfsMagic, "ntfs", true},
		{ntfs3Magic, "ntfs3", true},
		// ntfs-3g and exfat-fuse run in user space on a block device.
		{unix.FUSE_SUPER_MAGIC, "fuseblk", true},
		{unix.FUSE_SUPER_MAGIC, "fuse.sshfs", false},
		{unix.FUSE_SUPER_MAGIC, "fuse.shfs", false},
		{unix.EXT4_SUPER_MAGIC, "ext4", false},
		{unix.BTRFS_SUPER_MAGIC, "btrfs", false},
	}
	for _, c := range cases {
		if got := windowsFS(c.magic, c.fstype); got != c.want {
			t.Errorf("windowsFS(%#x, %s) = %v, want %v", c.magic, c.fstype, got, c.want)
		}
	}
}

// The numbers have to be the kernel's, as its headers name them.
func TestTheMagicNumbersAreTheKernels(t *testing.T) {
	header, err := os.ReadFile("/usr/include/linux/magic.h")
	if err != nil {
		t.Skip("no kernel headers here")
	}
	for name, want := range map[string]uint32{"MSDOS_SUPER_MAGIC": unix.MSDOS_SUPER_MAGIC, "EXFAT_SUPER_MAGIC": exfatMagic, "NTFS_SB_MAGIC": ntfsMagic, "FUSE_SUPER_MAGIC": unix.FUSE_SUPER_MAGIC} {
		m := regexp.MustCompile(`#define\s+` + name + `\s+(0x[0-9a-fA-F]+)`).FindSubmatch(header)
		if m == nil {
			t.Errorf("%s is not in magic.h", name)
			continue
		}
		got, err := strconv.ParseUint(string(m[1]), 0, 32)
		if err != nil || uint32(got) != want {
			t.Errorf("%s is %s in magic.h, here %#x", name, m[1], want)
		}
	}
}

func TestTheMountTableNamesTheTypeOfAFolder(t *testing.T) {
	mountinfo := "22 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw\n" +
		"40 22 8:17 / /media/anna/USB\\040STICK rw,nosuid - fuseblk /dev/sdb1 rw,user_id=0\n" +
		"41 22 8:33 / /media/anna/KAMERA rw - vfat /dev/sdc1 rw\n"
	for dir, want := range map[string]string{
		"/home/anna":                 "ext4",
		"/media/anna/USB STICK/Foto": "fuseblk",
		"/media/anna/KAMERA":         "vfat",
		"/media/anna/KAMERA2":        "ext4",
	} {
		if got := mountType(mountinfo, dir); got != want {
			t.Errorf("mountType(%s) = %q, want %q", dir, got, want)
		}
	}
}
