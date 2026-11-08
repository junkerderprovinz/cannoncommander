package winshell

import (
	"strings"

	"golang.org/x/sys/windows"
)

// LocalPath reports whether p, a path a shortcut or a desktop.ini names,
// stays on this computer. The shell opens a path on another computer with
// the user's credentials, which hands that computer the user's NTLM hash,
// so a listing must never follow one by itself.
func LocalPath(p string) bool {
	p = expand(p)
	switch {
	case p == "":
		return true
	case strings.HasPrefix(p, `\\`), strings.HasPrefix(p, "//"), strings.Contains(p, "://"):
		return false
	case len(p) >= 2 && p[1] == ':':
		root, err := windows.UTF16PtrFromString(p[:2] + `\`)
		return err == nil && windows.GetDriveType(root) != windows.DRIVE_REMOTE
	}
	return true
}

// expand fills in the environment variables a path names, such as
// %SystemRoot%.
func expand(p string) string {
	if !strings.Contains(p, "%") {
		return p
	}
	src, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.ExpandEnvironmentStrings(src, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return p
	}
	return windows.UTF16ToString(buf[:n])
}
