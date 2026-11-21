package update

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

const (
	shcneUpdateItem   = 0x00002000
	shcneAssocChanged = 0x08000000
	shcnfIDList       = 0x0000
	shcnfPathW        = 0x0005
)

// ShellChanged tells Explorer that the program at path was replaced. Without
// it, shortcuts and the Start menu keep the picture the shell cached for the
// program before, which can be the icon of a long gone version.
var ShellChanged = func(path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	procChangeNotify.Call(shcneUpdateItem, shcnfPathW, uintptr(unsafe.Pointer(p)), 0)
	procChangeNotify.Call(shcneAssocChanged, shcnfIDList, 0, 0)
}
