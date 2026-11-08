package fsapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// fixtureBase is a fresh folder on a local NTFS volume. Some tests need what
// only such a volume keeps or what Windows reads only from a local disk: the
// owner's account, the offline attribute of a cloud file and the name a
// desktop.ini gives a folder. A TEMP on a network share, as some machines
// keep it, offers none of that.
func fixtureBase(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	if localNTFS(dir) {
		return dir
	}
	local, err := os.MkdirTemp(filepath.Join(os.Getenv("LOCALAPPDATA"), "Temp"), "cc-fsapi-")
	if err != nil || !localNTFS(local) {
		t.Skipf("these tests need a local NTFS volume, and neither %s nor the profile's temp folder is one", dir)
	}
	t.Cleanup(func() { os.RemoveAll(local) })
	return local
}

func localNTFS(dir string) bool {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return false
	}
	root := make([]uint16, windows.MAX_PATH)
	if windows.GetVolumePathName(p, &root[0], uint32(len(root))) != nil {
		return false
	}
	if windows.GetDriveType(&root[0]) != windows.DRIVE_FIXED {
		return false
	}
	fs := make([]uint16, windows.MAX_PATH)
	if windows.GetVolumeInformation(&root[0], nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))) != nil {
		return false
	}
	return strings.EqualFold(windows.UTF16ToString(fs), "NTFS")
}
