//go:build !cdp

package main

// debugArgs is empty: only a build with the cdp tag lets a test drive the
// window, see cdp.go.
func debugArgs() []string { return nil }
