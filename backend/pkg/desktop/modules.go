//go:build desktop

package desktop

// Modules is what a desktop app offers that brings its helper programs
// itself: ffmpeg, 7-Zip and MuPDF's mutool. The server finds it on Current.
type Modules interface {
	ModulesState() ModulesState
	// FetchModules starts fetching the modules of the app's version and
	// returns at once. It returns errors.ErrUnsupported where the app does
	// not fetch them, as a Mac bundle and a Flatpak bring theirs.
	FetchModules() error
}

// ModulesState is how fetching the modules goes.
type ModulesState struct {
	// Offered says the app can fetch its modules.
	Offered  bool    `json:"offered"`
	Running  bool    `json:"running"`
	Fraction float64 `json:"fraction"`
	// Ready says modules came in since the app started. They are on the
	// PATH, and ffmpeg takes over after a restart.
	Ready  bool `json:"ready"`
	Failed bool `json:"failed"`
}
