package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/winshell"
)

// DragFiles runs the shell's drag loop on the window's thread, where the
// drag image and the pointer belong. The page starts it while the mouse
// button is still down, so the drag picks up where the pointer is.
func (*shell) DragFiles(paths []string) error {
	return application.InvokeSyncWithError(func() error { return winshell.DragFiles(winshell.Owner(), paths) })
}
