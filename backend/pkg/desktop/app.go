// Package desktop is where the desktop app plugs into the server it runs in
// its own process. Built without the desktop tag, as the container's server
// is, it keeps only the answers a server on its own gives.
package desktop

// App is what the desktop app offers the server.
type App interface {
	// AutoUpdate reports whether the app may put a newer release in place by
	// itself, and SetAutoUpdate changes that.
	AutoUpdate() bool
	SetAutoUpdate(on bool) error
	// UpdateReady is the version waiting for the next start, or "".
	UpdateReady() string
	// Restart ends the app and starts it again.
	Restart()
}
