package elevate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
)

func init() {
	recycle = func(path string) error {
		b, err := sysbin.Open()
		if err != nil {
			return err
		}
		_, err = b.Put(path)
		return err
	}
}

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellExecuteInfo is SHELLEXECUTEINFOW in its 64 bit layout.
type shellExecuteInfo struct {
	size     uint32
	mask     uint32
	hwnd     uintptr
	verb     *uint16
	file     *uint16
	params   *uint16
	dir      *uint16
	show     int32
	instApp  uintptr
	idList   uintptr
	class    *uint16
	keyClass uintptr
	hotKey   uint32
	icon     uintptr
	process  windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x40
	seeMaskNoAsync        = 0x100
	seeMaskFlagNoUI       = 0x400
)

// launchHelper starts this program again with the verb runas, which brings
// up the UAC prompt and returns once it is answered. The secret goes on the
// command line, which a process without administrator rights cannot read
// from an elevated one.
func launchHelper(_ context.Context, port int, secret []byte) (func() error, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil || errors.Is(err, syscall.Errno(1)) {
		defer windows.CoUninitialize()
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	params, _ := windows.UTF16PtrFromString(fmt.Sprintf("--elevated %d:%x", port, secret))
	info := shellExecuteInfo{mask: seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI, verb: verb, file: file, params: params, show: windows.SW_HIDE}
	info.size = uint32(unsafe.Sizeof(info))
	if ok, _, err := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return nil, ErrDeclined
		}
		return nil, err
	}
	if info.process == 0 {
		return nil, ErrNoHelper
	}
	return func() error {
		defer windows.CloseHandle(info.process)
		if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
			return err
		}
		var code uint32
		if err := windows.GetExitCodeProcess(info.process, &code); err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("the helper ended with %d", code)
		}
		return nil
	}, nil
}
