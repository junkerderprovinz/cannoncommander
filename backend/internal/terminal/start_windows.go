package terminal

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// startNative runs l and leaves it running. A console program goes through
// CreateProcess itself: os/exec hands every child the null device for its
// input, and cmd or PowerShell reading from it would end at once instead of
// waiting in their new window.
func startNative(l launch) error {
	if !l.console {
		cmd := exec.Command(l.program, l.args...)
		cmd.Dir = l.dir
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{l.program}, l.args...)))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(l.dir)
	if err != nil {
		return err
	}
	si := &windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, line, nil, nil, false, windows.CREATE_NEW_CONSOLE, nil, dir, si, &pi); err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}
