//go:build desktop

package launch

import (
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

type shellSystem struct{}

func newSystem() system { return shellSystem{} }

// asks is false: Explorer starts a program on a double click, and the shell
// warns on its own about one that came from the internet.
func (shellSystem) asks(string) bool { return false }

func (shellSystem) open(file string) error { return shellExecute(file) }

func (shellSystem) run(file string) error { return shellExecute(file) }

// shellExecute opens file with its default verb, as Explorer does. The shell
// may hand the call to extensions that need a single-threaded apartment.
func shellExecute(file string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == windows.Errno(windows.S_FALSE) {
		defer windows.CoUninitialize()
	}
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(file))
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, nil, f, nil, dir, windows.SW_SHOWNORMAL)
}
