//go:build desktop

package desktop

// Window is the app's main window, for the title bar the page draws where
// the window has no frame of its own. The server finds it on Current.
type Window interface {
	WindowMaximised() bool
	MaximiseWindow(on bool)
	MinimiseWindow()
	// CloseWindow asks as the frame's close button does, so a window that
	// waits in the tray hides instead.
	CloseWindow()
}
