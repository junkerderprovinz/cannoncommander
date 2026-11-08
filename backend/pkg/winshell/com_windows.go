package winshell

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")
)

// Slots of IUnknown, which every COM interface starts with.
const (
	slotQueryInterface = 0
	slotRelease        = 2
)

// HRESULT is the error a COM call failed with.
type HRESULT uint32

func (h HRESULT) Error() string { return fmt.Sprintf("HRESULT %#08x", uint32(h)) }

// Failed turns what a COM call returned into an error, or nil on success.
func Failed(r uintptr) error {
	if int32(r) < 0 {
		return HRESULT(r)
	}
	return nil
}

// Call calls slot n of the vtable of the COM object obj with args.
func Call(obj unsafe.Pointer, n int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(n)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}

// Release lets go of obj, which may be nil.
func Release(obj unsafe.Pointer) {
	if obj != nil {
		Call(obj, slotRelease)
	}
}

// Query asks obj for another of its interfaces.
func Query(obj unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	if err := Failed(Call(obj, slotQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))); err != nil {
		return nil, err
	}
	return out, nil
}

// TaskString reads the string slot n of obj hands out, which the caller
// frees.
func TaskString(obj unsafe.Pointer, n int) (string, error) {
	var p *uint16
	if err := Failed(Call(obj, n, uintptr(unsafe.Pointer(&p)))); err != nil {
		return "", err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p), nil
}

// Apartment runs fn on a thread of its own in a single-threaded apartment,
// which the shell's handlers, menus and dialogs expect from their caller.
func Apartment(fn func() error) error {
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

// Owner is the app's window while it is in front, which it is right after
// the person clicked in it, so a menu or a dialog opens over it. It is 0
// when another program's window is in front.
func Owner() windows.HWND {
	w := windows.GetForegroundWindow()
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(w, &pid); err != nil || int(pid) != os.Getpid() {
		return 0
	}
	return w
}
