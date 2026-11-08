package nativeshell

import (
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	shlwapi = windows.NewLazySystemDLL("shlwapi.dll")

	procMultiFileProperties = shell32.NewProc("SHMultiFileProperties")
	procAssocQueryString    = shlwapi.NewProc("AssocQueryStringW")
	procEnumThreadWindows   = user32.NewProc("EnumThreadWindows")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
)

// HasProperties reports that Windows has a properties dialog.
func HasProperties() bool { return true }

// Properties opens Explorer's properties dialog for files, which have to
// share a folder, with its tabs for security, details and previous
// versions. It returns once the dialog is open.
func Properties(files []string) error {
	return keepAnswering(func() error {
		sel, err := winshell.Select(files)
		if err != nil {
			return err
		}
		defer sel.Close()
		obj, err := sel.DataObject()
		if err != nil {
			return err
		}
		defer winshell.Release(obj)
		r, _, _ := procMultiFileProperties.Call(uintptr(obj), 0)
		return winshell.Failed(r)
	})
}

// CanRunAsAdmin reports whether Windows offers to start file as
// administrator, which it does for programs and scripts it can run.
func CanRunAsAdmin(file string) bool {
	ext, err := windows.UTF16PtrFromString(filepath.Ext(file))
	if err != nil || filepath.Ext(file) == "" {
		return false
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	const assocStrCommand = 1
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	r, _, _ := procAssocQueryString.Call(0, assocStrCommand, uintptr(unsafe.Pointer(ext)), uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	return r == 0
}

// RunAsAdmin starts file with administrator rights. Windows asks the person
// first, and a no there is no failure.
func RunAsAdmin(file string) error {
	if !CanRunAsAdmin(file) {
		return ErrUnsupported
	}
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(file))
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	err = winshell.Execute(&winshell.ExecuteInfo{Mask: winshell.SeeMaskNoAsync, Verb: verb, File: f, Dir: dir, Show: windows.SW_SHOWNORMAL})
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return nil
	}
	return err
}

// keepAnswering runs fn in a single-threaded apartment and returns its
// error, then keeps the thread answering its messages while it shows a
// window, since a dialog the shell opens there lives on it.
func keepAnswering(fn func() error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil {
			done <- err
			return
		}
		defer windows.CoUninitialize()
		err := fn()
		done <- err
		if err == nil {
			pump(5 * time.Second)
		}
	}()
	return <-done
}

var (
	countOnce   sync.Once
	countWindow uintptr
)

// showsWindows reports whether the calling thread has a visible window.
func showsWindows() bool {
	countOnce.Do(func() {
		countWindow = windows.NewCallback(func(hwnd windows.HWND, found *bool) uintptr {
			if v, _, _ := procIsWindowVisible.Call(uintptr(hwnd)); v != 0 {
				*found = true
				return 0
			}
			return 1
		})
	})
	found := false
	procEnumThreadWindows.Call(uintptr(windows.GetCurrentThreadId()), countWindow, uintptr(unsafe.Pointer(&found)))
	return found
}
