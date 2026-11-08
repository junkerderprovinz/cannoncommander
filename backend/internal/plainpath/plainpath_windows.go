package plainpath

import (
	"io/fs"
	"syscall"

	"golang.org/x/sys/windows"
)

// Link reports whether fi, from Lstat, is a link, a junction or any other
// reparse point, all of which can lead somewhere else.
func Link(fi fs.FileInfo) bool {
	if fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return true
	}
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return ok && d.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// Mine reports whether p belongs to the account running the program. The
// caller checks with Link first, since the owner of a reparse point's
// target is read here.
func Mine(p string) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return false, err
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false, err
	}
	return owner.Equals(u.User.Sid), nil
}
