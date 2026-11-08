//go:build !linux

package main

// outdated is false: the sources a first start writes on Windows and on a
// Mac have stayed the same.
func outdated([]storedSource) bool { return false }
