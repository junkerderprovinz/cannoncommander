// Package flatpak tells whether the program runs inside a Flatpak sandbox.
// There the software centre updates it, it owns bus names under its own ID
// alone, and the computer's own programs, settings and system folders sit
// outside, the system folders under /run/host.
package flatpak

import "os"

// ID returns the application ID the program runs under inside a Flatpak, or
// "" outside one. Flatpak sets FLATPAK_ID for every program it starts.
func ID() string { return os.Getenv("FLATPAK_ID") }
