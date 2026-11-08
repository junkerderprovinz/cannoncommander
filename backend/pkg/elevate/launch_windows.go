package elevate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/sysbin"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
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

// launchHelper starts this program again with the verb runas, which brings
// up the UAC prompt and returns once it is answered. The secret goes on the
// command line, which a process without administrator rights cannot read
// from an elevated one.
func launchHelper(_ context.Context, port int, secret []byte) (func() error, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	params, _ := windows.UTF16PtrFromString(fmt.Sprintf("--elevated %d:%x", port, secret))
	info := winshell.ExecuteInfo{Mask: winshell.SeeMaskNoCloseProcess | winshell.SeeMaskNoAsync | winshell.SeeMaskFlagNoUI, Verb: verb, File: file, Params: params, Show: windows.SW_HIDE}
	err = winshell.Execute(&info)
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return nil, ErrDeclined
	}
	if err != nil {
		return nil, err
	}
	if info.Process == 0 {
		return nil, ErrNoHelper
	}
	return func() error {
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
}
