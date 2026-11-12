package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procAllocConsoleWithOptions = kernel32.NewProc("AllocConsoleWithOptions")
	procAttachConsole           = kernel32.NewProc("AttachConsole")
)

const allocConsoleNoWindow = 2

// quietConsole gives the program a console without a window. A desktop
// program has none, so every console program it starts, its own helpers and
// the ones the ffmpeg library runs for each thumbnail, would open a window.
// They share this one instead. What a person starts asks for a console of its
// own, see launch and openwith.
func quietConsole() {
	if procAllocConsoleWithOptions.Find() == nil {
		opts := struct {
			mode          int32
			useShowWindow int32
			showWindow    uint16
		}{mode: allocConsoleNoWindow}
		var result int32
		if hr, _, _ := procAllocConsoleWithOptions.Call(uintptr(unsafe.Pointer(&opts)), uintptr(unsafe.Pointer(&result))); hr == 0 {
			return
		}
	}
	// Before Windows 11 24H2 a console without a window belongs to a console
	// program started without one, and this program joins it. cmd waits for
	// input and ends once the pipe closes; the console stays with its member.
	cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/d", "/q", "/k")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	in, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	// The console is there a few milliseconds after the process.
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if ok, _, _ := procAttachConsole.Call(uintptr(cmd.Process.Pid)); ok != 0 {
			break
		}
	}
	in.Close()
	_ = cmd.Wait()
}
