package winshell

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteEx = shell32.NewProc("ShellExecuteExW")

// Flags of ExecuteInfo.Mask.
const (
	SeeMaskNoCloseProcess = 0x40
	SeeMaskNoAsync        = 0x100
	SeeMaskFlagNoUI       = 0x400
)

// ExecuteInfo is SHELLEXECUTEINFOW in its 64 bit layout. Execute fills in
// its size and its window.
type ExecuteInfo struct {
	size    uint32
	Mask    uint32
	hwnd    windows.HWND
	Verb    *uint16
	File    *uint16
	Params  *uint16
	Dir     *uint16
	Show    int32
	instApp windows.Handle
	idList  uintptr
	class   *uint16
	key     windows.Handle
	hotKey  uint32
	icon    windows.Handle
	// Process is the started program, with SeeMaskNoCloseProcess, which the
	// caller closes.
	Process windows.Handle
}

// Execute starts what info names the way Explorer does, from a thread in
// the shell's apartment. The app's window owns any prompt it brings up,
// such as the one for administrator rights.
func Execute(info *ExecuteInfo) error {
	info.size = uint32(unsafe.Sizeof(*info))
	return Apartment(func() error {
		info.hwnd = Owner()
		if ok, _, err := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info))); ok == 0 {
			return err
		}
		return nil
	})
}
