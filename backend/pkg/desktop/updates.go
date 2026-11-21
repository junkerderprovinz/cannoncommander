//go:build desktop

package desktop

// Updates is what a desktop app offers that fetches its new version on
// request. The server finds it on Current.
type Updates interface {
	UpdateState() UpdateState
	// UpdateNow starts fetching the newest release and returns at once. It
	// returns errors.ErrUnsupported where the app does not replace itself, as
	// a Flatpak, which the software centre updates.
	UpdateNow() error
}

// UpdateState is how an update started by hand goes.
type UpdateState struct {
	Busy bool `json:"busy"`
	// Percent is how much of the download is in, or -1 while that is unknown.
	Percent int `json:"percent"`
	// Ready is the version in place for the next start.
	Ready string `json:"ready,omitempty"`
	// Failed says why the last update started by hand did not happen.
	Failed string `json:"failed,omitempty"`
}
