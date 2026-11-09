package nativeshell

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/shortcut"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

// reachesOut reports whether the shell would fetch the icon of file from
// another computer, as a shortcut, a web link or the desktop.ini of a
// folder may ask it to. A shortcut that cannot be read counts as one that
// does, since the shell might still make sense of it.
func reachesOut(file string) bool {
	info, err := os.Stat(file)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return folderReachesOut(file)
	}
	kind := shortcut.KindOf(file)
	if kind != shortcut.Lnk && kind != shortcut.URL {
		return false
	}
	s, err := shortcut.Read(file)
	if err != nil {
		return true
	}
	if s.Icon != "" || kind == shortcut.URL {
		return !winshell.LocalPath(withoutIndex(s.Icon))
	}
	return !winshell.LocalPath(s.Target)
}

// folderReachesOut reads the icon a folder's desktop.ini names. Explorer
// reads that file only in a folder marked read-only or system.
func folderReachesOut(dir string) bool {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil || attrs&(windows.FILE_ATTRIBUTE_READONLY|windows.FILE_ATTRIBUTE_SYSTEM) == 0 {
		return false
	}
	f, err := os.Open(filepath.Join(dir, "desktop.ini"))
	if err != nil {
		return false
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 16<<10))
	if err != nil {
		return true
	}
	sc := bufio.NewScanner(bytes.NewReader(iniText(raw)))
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "iconresource", "iconfile":
			if !winshell.LocalPath(withoutIndex(strings.TrimSpace(value))) {
				return true
			}
		}
	}
	return false
}

// iniText is a desktop.ini as UTF-8. Windows writes it in UTF-16 with a byte
// order mark, or in the code page of the computer.
func iniText(b []byte) []byte {
	if len(b) < 2 || b[0] != 0xff || b[1] != 0xfe {
		return b
	}
	u := make([]uint16, (len(b)-2)/2)
	for i := range u {
		u[i] = uint16(b[2+2*i]) | uint16(b[3+2*i])<<8
	}
	return []byte(string(utf16.Decode(u)))
}

// withoutIndex drops the ",index" an icon location ends with.
func withoutIndex(loc string) string {
	at := strings.LastIndexByte(loc, ',')
	if at < 0 {
		return loc
	}
	if _, err := strconv.Atoi(strings.TrimSpace(loc[at+1:])); err != nil {
		return loc
	}
	return loc[:at]
}
