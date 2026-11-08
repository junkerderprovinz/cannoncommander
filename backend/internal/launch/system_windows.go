//go:build desktop

package launch

import (
	"path/filepath"
	"runtime"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
	"golang.org/x/sys/windows"
)

type shellSystem struct{}

func newSystem() system { return shellSystem{} }

// asks is false: Explorer starts a program on a double click, and the shell
// warns on its own about one that came from the internet.
func (shellSystem) asks(string) bool { return false }

func (shellSystem) open(file string) error {
	if to := toOpen(file); to != file {
		return shellExecute(to, "")
	}
	return shellExecute(file, filepath.Dir(file))
}

func (shellSystem) run(file string) error { return shellExecute(file, filepath.Dir(file)) }

// shellExecute opens file with its default verb, as Explorer does. The shell
// may hand the call to extensions that need a single-threaded apartment. A
// warning it shows, such as for a file from a share, belongs to the app's
// window, or it would open behind it.
func shellExecute(file, dir string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == windows.Errno(windows.S_FALSE) {
		defer windows.CoUninitialize()
	}
	f, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var in *uint16
	if dir != "" {
		if in, err = windows.UTF16PtrFromString(dir); err != nil {
			return err
		}
	}
	return windows.ShellExecute(windows.Handle(winshell.Owner()), nil, f, nil, in, windows.SW_SHOWNORMAL)
}
