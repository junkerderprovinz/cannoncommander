//go:build !windows

package archives

// installed finds nothing outside Windows, where the packages put their
// programs on the PATH.
var installed = func(string) string { return "" }
