package openwith

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	shlwapi = windows.NewLazySystemDLL("shlwapi.dll")
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")

	procEnumHandlers       = shell32.NewProc("SHAssocEnumHandlers")
	procItemFromPath       = shell32.NewProc("SHCreateItemFromParsingName")
	procOpenWithDialog     = shell32.NewProc("SHOpenWithDialog")
	procExtractIcon        = shell32.NewProc("SHDefExtractIconW")
	procAssocQueryString   = shlwapi.NewProc("AssocQueryStringW")
	procLoadIndirectString = shlwapi.NewProc("SHLoadIndirectString")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetObject          = gdi32.NewProc("GetObjectW")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

var (
	iidShellItem   = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
	iidDataObject  = windows.GUID{Data1: 0x0000010e, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	bhidDataObject = windows.GUID{Data1: 0xb8c0bd9f, Data2: 0xed24, Data3: 0x455c, Data4: [8]byte{0x83, 0xe6, 0xd5, 0x39, 0x0c, 0x4f, 0xe8, 0xc4}}
)

// Slots in the vtables of the interfaces used here, counted from IUnknown's
// three.
const (
	release = 2

	enumNext = 3

	handlerName          = 3
	handlerUIName        = 4
	handlerIconLocation  = 5
	handlerInvoke        = 8
	shellItemBindHandler = 3
)

// hresult is the error a COM call failed with.
type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("HRESULT %#08x", uint32(h)) }

func failed(r uintptr) error {
	if int32(r) < 0 {
		return hresult(r)
	}
	return nil
}

// method calls slot n of the COM object obj.
func method(obj unsafe.Pointer, n int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(n)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

func releaseObject(obj unsafe.Pointer) {
	if obj != nil {
		method(obj, release)
	}
}

// taskString reads a string the shell allocated for the caller and frees it.
func taskString(obj unsafe.Pointer, n int) (string, error) {
	var p *uint16
	if err := failed(method(obj, n, uintptr(unsafe.Pointer(&p)))); err != nil {
		return "", err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p), nil
}

// apartment runs fn on a thread of its own in a single-threaded apartment,
// which the shell's handlers and its dialog expect from their caller.
func apartment(fn func() error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil {
			done <- err
			return
		}
		defer windows.CoUninitialize()
		done <- fn()
	}()
	return <-done
}

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
	if err := failed(r); err != nil {
		return err
	}
	defer releaseObject(enum)
	for {
		var handler unsafe.Pointer
		var got uint32
		if method(enum, enumNext, 1, uintptr(unsafe.Pointer(&handler)), uintptr(unsafe.Pointer(&got))) != 0 || got == 0 {
			return nil
		}
		stop := fn(handler)
		releaseObject(handler)
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
	if err := failed(r); err != nil {
		return nil, err
	}
	defer releaseObject(item)
	var obj unsafe.Pointer
	if err := failed(method(item, shellItemBindHandler, 0, uintptr(unsafe.Pointer(&bhidDataObject)), uintptr(unsafe.Pointer(&iidDataObject)), uintptr(unsafe.Pointer(&obj)))); err != nil {
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
