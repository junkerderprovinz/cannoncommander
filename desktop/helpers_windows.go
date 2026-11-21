package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

// fetchElevated runs this program with --modules and administrator rights,
// which the folder of the copy installed for all users asks for, and waits
// for it. Windows asks for those rights first.
func fetchElevated() error { return runElevated("--modules") }

// updateElevated runs the scheduled task's update at once, for "Update now".
func updateElevated() error { return runElevated("--update --now") }

// runElevated runs this program with args and administrator rights and waits
// for it to end.
func runElevated(args string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, _ := windows.UTF16PtrFromString(args)
	info := winshell.ExecuteInfo{Mask: winshell.SeeMaskNoCloseProcess | winshell.SeeMaskNoAsync | winshell.SeeMaskFlagNoUI, Verb: verb, File: file, Params: params, Show: windows.SW_HIDE}
	if err := winshell.Execute(&info); err != nil {
		return err
	}
	if info.Process == 0 {
		return fmt.Errorf("%s did not start", args)
	}
	defer windows.CloseHandle(info.Process)
	if _, err := windows.WaitForSingleObject(info.Process, windows.INFINITE); err != nil {
		return err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.Process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("%s ended with %d", args, code)
	}
	return nil
}
