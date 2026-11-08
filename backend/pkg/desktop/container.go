//go:build !desktop

package desktop

import "net/http"

// Current returns nil: no desktop app runs the container's server.
func Current() App { return nil }

// Active reports false, so the compiler drops what only the desktop app does.
func Active() bool { return false }

// Home returns "", the panels open where the server's sources say.
func Home() string { return "" }

// Guard returns next, since the container has no window to keep apart.
func Guard(next http.Handler) http.Handler { return next }

// Listen does nothing, as no desktop app emits events.
func Listen(func(typ string, data any)) {}
