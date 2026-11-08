//go:build !windows && !linux

package ccfs

// WindowsNames reports whether names in dir follow the rules of Windows.
func WindowsNames(string) bool { return false }
