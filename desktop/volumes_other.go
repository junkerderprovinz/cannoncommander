//go:build !windows && !linux

package main

// volumeWatch is empty on a Mac, where the app does not follow the drives:
// /Volumes is a source and every disk shows up in it.
type volumeWatch struct{}
