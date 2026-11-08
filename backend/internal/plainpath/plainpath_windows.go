package plainpath

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Link reports whether the entry at path, which fi describes from Lstat, is
// a link or a junction: a reparse point whose tag says it stands for another
// name. Cloud placeholders, deduplicated files and the like are reparse points
// too, but plain entries with their own data.
func Link(fi fs.FileInfo, path string) bool {
	if fi.Mode()&fs.ModeSymlink != 0 {
		return true
	}
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok || d.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false
	}
	h, err := openNoFollow(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if err != nil {
		return true
	}
	defer windows.CloseHandle(h)
	surrogate, _, err := tagOf(h)
	return err != nil || surrogate
}

// nameSurrogate is IsReparseTagNameSurrogate.
func nameSurrogate(tag uint32) bool { return tag&0x20000000 != 0 }

func openNoFollow(path string, access, share uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, access, share, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

// tagOf reports whether the entry behind h is a name surrogate, and whether
// it is a folder.
func tagOf(h windows.Handle) (surrogate, dir bool, err error) {
	var info struct{ FileAttributes, ReparseTag uint32 }
	if err := windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return false, false, err
	}
	surrogate = info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && nameSurrogate(info.ReparseTag)
	return surrogate, info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0, nil
}

// Mine reports whether p belongs to the account running the program, or to
// the Administrators group when the account is one of them: only an
// administrator can have given it to that group, and a folder made while
// the app ran with administrator rights belongs to it. The caller checks
// with Link first, since the owner of a reparse point's target is read here.
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
	if owner.Equals(u.User.Sid) {
		return true, nil
	}
	return owner.IsWellKnown(windows.WinBuiltinAdministratorsSid) && admin(), nil
}

// admin reports whether the account belongs to the Administrators group. A
// token without administrator rights still lists the group, only for
// denying, so its presence is what counts.
func admin() bool {
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.AllGroups() {
		if g.Sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
			return true
		}
	}
	return false
}

// Hold opens every folder above path, from the top of its volume, refusing
// a link on the way, and keeps them open without letting anybody delete or
// rename them until release is called. Work on path in between cannot be
// led elsewhere by swapping a folder for a junction.
func Hold(path string) (release func(), err error) {
	path = filepath.Clean(path)
	vol := filepath.VolumeName(path)
	var held []windows.Handle
	release = func() {
		for i := len(held) - 1; i >= 0; i-- {
			windows.CloseHandle(held[i])
		}
	}
	at := vol + string(filepath.Separator)
	dirs := []string{at}
	parent := filepath.Dir(path)
	for _, part := range strings.Split(strings.TrimPrefix(parent[len(vol):], string(filepath.Separator)), string(filepath.Separator)) {
		if part != "" {
			at = filepath.Join(at, part)
			dirs = append(dirs, at)
		}
	}
	for _, d := range dirs {
		// Opening for attributes alone would skip the share check, so the
		// folder is opened for listing.
		h, err := openNoFollow(d, windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
		if err != nil {
			release()
			return nil, &fs.PathError{Op: "hold", Path: d, Err: err}
		}
		held = append(held, h)
		surrogate, dir, err := tagOf(h)
		if err == nil && (surrogate || !dir) {
			err = ErrLink
		}
		if err != nil {
			release()
			return nil, &fs.PathError{Op: "hold", Path: d, Err: err}
		}
	}
	return release, nil
}
