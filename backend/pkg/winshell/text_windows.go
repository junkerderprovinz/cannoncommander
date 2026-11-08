package winshell

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procLoadIndirectString = windows.NewLazySystemDLL("shlwapi.dll").NewProc("SHLoadIndirectString")

// IndirectString reads a text Windows keeps in a program, which the registry
// and desktop.ini point to as "@%SystemRoot%\system32\shell32.dll,-21770".
// It gives the text in the language of the system, and any other string as
// it is.
func IndirectString(s string) (string, error) {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, 1024)
	r, _, _ := procLoadIndirectString.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if err := Failed(r); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}
