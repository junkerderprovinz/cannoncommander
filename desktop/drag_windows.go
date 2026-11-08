package main

import (
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

var (
	ole32 = windows.NewLazySystemDLL("ole32.dll")

	procOleInitialize = ole32.NewProc("OleInitialize")
	procDoDragDrop    = shell32.NewProc("SHDoDragDrop")
)

// oleOnce sets up OLE on the window's thread, which drag and drop needs.
// The thread keeps it for as long as the app runs.
var oleOnce sync.Once

// DragFiles runs the shell's drag loop on the window's thread, where the
// drag image and the pointer belong. The page starts it while the mouse
// button is still down, so the drag picks up where the pointer is.
func (*shell) DragFiles(paths []string) error {
	return application.InvokeSyncWithError(func() error {
		oleOnce.Do(func() { procOleInitialize.Call(0) })
		sel, err := winshell.Select(paths)
		if err != nil {
			return err
		}
		defer sel.Close()
		data, err := sel.DataObject()
		if err != nil {
			return err
		}
		defer winshell.Release(data)
		var effect uint32
		r, _, _ := procDoDragDrop.Call(uintptr(winshell.Owner()), uintptr(data), 0, dropCopy|dropMove|dropLink, uintptr(unsafe.Pointer(&effect)))
		return winshell.Failed(r)
	})
}
