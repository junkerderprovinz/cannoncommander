// Package winres reads the texts Windows keeps in its programs, which the
// registry and desktop.ini point to with a leading @.
package winres

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var loadIndirect = windows.NewLazySystemDLL("shlwapi.dll").NewProc("SHLoadIndirectString")

// String turns "@%SystemRoot%\system32\shell32.dll,-21770" into the text
// in the language of the system, and returns any other s as it is. A
// reference that leads nowhere gives "".
func String(s string) string {
	if !strings.HasPrefix(s, "@") {
		return s
	}
	src, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return ""
	}
	out := make([]uint16, 512)
	if r, _, _ := loadIndirect.Call(uintptr(unsafe.Pointer(src)), uintptr(unsafe.Pointer(&out[0])), uintptr(len(out)), 0); r != 0 {
		return ""
	}
	return windows.UTF16ToString(out)
}
