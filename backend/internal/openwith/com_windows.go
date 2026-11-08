//go:build desktop

package openwith

import (
	"unsafe"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	shlwapi = windows.NewLazySystemDLL("shlwapi.dll")

	procEnumHandlers       = shell32.NewProc("SHAssocEnumHandlers")
	procItemFromPath       = shell32.NewProc("SHCreateItemFromParsingName")
	procOpenWithDialog     = shell32.NewProc("SHOpenWithDialog")
	procAssocQueryString   = shlwapi.NewProc("AssocQueryStringW")
	procLoadIndirectString = shlwapi.NewProc("SHLoadIndirectString")
)

var (
	iidShellItem   = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
	bhidDataObject = windows.GUID{Data1: 0xb8c0bd9f, Data2: 0xed24, Data3: 0x455c, Data4: [8]byte{0x83, 0xe6, 0xd5, 0x39, 0x0c, 0x4f, 0xe8, 0xc4}}
)

// Slots in the vtables of the interfaces used here, counted from IUnknown's
// three.
const (
	enumNext = 3

	handlerName          = 3
	handlerUIName        = 4
	handlerIconLocation  = 5
	handlerInvoke        = 8
	shellItemBindHandler = 3
)

// eachHandler calls fn with each program Windows recommends for files that
// end in ext, until fn returns true. The handler is released after fn.
func eachHandler(ext string, fn func(handler unsafe.Pointer) bool) error {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return err
	}
	const recommended = 1
	var enum unsafe.Pointer
	r, _, _ := procEnumHandlers.Call(uintptr(unsafe.Pointer(p)), recommended, uintptr(unsafe.Pointer(&enum)))
	if err := winshell.Failed(r); err != nil {
		return err
	}
	defer winshell.Release(enum)
	for {
		var handler unsafe.Pointer
		var got uint32
		if winshell.Call(enum, enumNext, 1, uintptr(unsafe.Pointer(&handler)), uintptr(unsafe.Pointer(&got))) != 0 || got == 0 {
			return nil
		}
		stop := fn(handler)
		winshell.Release(handler)
		if stop {
			return nil
		}
	}
}

// dataObject is the file as the shell hands it to a program.
func dataObject(file string) (unsafe.Pointer, error) {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return nil, err
	}
	var item unsafe.Pointer
	r, _, _ := procItemFromPath.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if err := winshell.Failed(r); err != nil {
		return nil, err
	}
	defer winshell.Release(item)
	var obj unsafe.Pointer
	if err := winshell.Failed(winshell.Call(item, shellItemBindHandler, 0, uintptr(unsafe.Pointer(&bhidDataObject)), uintptr(unsafe.Pointer(&winshell.IIDDataObject)), uintptr(unsafe.Pointer(&obj)))); err != nil {
		return nil, err
	}
	return obj, nil
}

// assocString asks Windows about the program that opens files ending in ext.
func assocString(ext string, what uint32) string {
	p, err := windows.UTF16PtrFromString(ext)
	if err != nil {
		return ""
	}
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	r, _, _ := procAssocQueryString.Call(0, uintptr(what), uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
