//go:build !windows && !linux

package main

// newFolderHandler is nil on a Mac, where the Finder keeps its folders.
func newFolderHandler() folderHandler { return nil }

func offerFolders(*shell, bool) {}
