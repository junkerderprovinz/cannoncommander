package desktop

// FileManager is what a desktop app offers that can take the place of the
// computer's own file manager. The server finds it on Current. Where the app
// cannot, both methods return errors.ErrUnsupported.
type FileManager interface {
	// IsFileManager reports whether the computer opens folders in the app.
	IsFileManager() (bool, error)
	// SetFileManager makes the app the computer's file manager, named with
	// words where the computer shows it, or gives the place back.
	SetFileManager(on bool, words Words) error
}

// Words are the app's texts for places outside its window, in the language
// the person reads the app in.
type Words struct {
	// Open labels the app's entry in the context menu of a folder.
	Open string `json:"open"`
	// Show and Quit label the menu of the app's icon in the tray.
	Show string `json:"show"`
	Quit string `json:"quit"`
}
