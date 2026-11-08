package elevate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/plainpath"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

// put puts path in the Recycle Bin.
var put = func(path string) error {
	b, err := sysbin.Open()
	if err != nil {
		return err
	}
	_, err = b.Put(path)
	return err
}

// The shell works by path, so the folders above path are held open while it
// does, and none of them can be renamed or swapped for a junction meanwhile.
func init() {
	recycle = func(path string) error {
		release, err := plainpath.Hold(path)
		if err != nil {
			return err
		}
		defer release()
		return put(path)
	}
}

// launchHelper starts this program again with the verb runas, which brings
// up the UAC prompt and returns once it is answered, and talks to it over a
// pipe.
func launchHelper(ctx context.Context) (io.ReadWriteCloser, []byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	return overPipe(ctx, func(value string) (uint32, func() error, error) {
		verb, _ := windows.UTF16PtrFromString("runas")
		file, err := windows.UTF16PtrFromString(exe)
		if err != nil {
			return 0, nil, err
		}
		params, err := windows.UTF16PtrFromString("--elevated " + value)
		if err != nil {
			return 0, nil, err
		}
		info := winshell.ExecuteInfo{Mask: winshell.SeeMaskNoCloseProcess | winshell.SeeMaskNoAsync | winshell.SeeMaskFlagNoUI, Verb: verb, File: file, Params: params, Show: windows.SW_HIDE}
		err = winshell.Execute(&info)
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return 0, nil, ErrDeclined
		}
		if err != nil {
			return 0, nil, err
		}
		if info.Process == 0 {
			return 0, nil, ErrNoHelper
		}
		pid, err := windows.GetProcessId(info.Process)
		if err != nil {
			windows.CloseHandle(info.Process)
			return 0, nil, err
		}
		return pid, func() error {
			defer windows.CloseHandle(info.Process)
			if _, err := windows.WaitForSingleObject(info.Process, windows.INFINITE); err != nil {
				return err
			}
			var code uint32
			if err := windows.GetExitCodeProcess(info.Process, &code); err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("the helper ended with %d", code)
			}
			return nil
		}, nil
	})
}
