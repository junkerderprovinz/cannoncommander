package desktop

// Clipboard is the system clipboard as the desktop app shares files through
// it with Explorer and the file managers of Linux. Paths are paths on disk.
type Clipboard interface {
	// ClipboardFiles returns the files on the clipboard and whether they were
	// cut. A clipboard that holds no files gives none and no error.
	ClipboardFiles() (paths []string, cut bool, err error)
	// CopyFiles puts files on the clipboard the way the system's file
	// manager does, marked to be moved when cut.
	CopyFiles(paths []string, cut bool) error
	// ClearClipboard empties the clipboard, as a file manager does once it
	// moved the files that were cut, so the program they came from stops
	// showing them as cut.
	ClearClipboard() error
}

// Dragger hands files to the system's drag and drop, so they can be dropped
// on another program.
type Dragger interface {
	// DragFiles starts the drag where the pointer is and returns once the
	// files are dropped or the drag is given up.
	DragFiles(paths []string) error
}

// SystemClipboard returns the desktop app's clipboard, or nil where there is
// none: in the container, and on a system the app cannot reach it on.
func SystemClipboard() Clipboard {
	c, _ := Current().(Clipboard)
	return c
}

// SystemDrag returns what drags files out of the desktop app's window, or nil.
func SystemDrag() Dragger {
	d, _ := Current().(Dragger)
	return d
}
