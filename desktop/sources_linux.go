package main

import (
	"os"
	"path/filepath"
	"slices"
)

// defaultSources is the whole computer from /, as every file manager on Linux
// shows it, with the panels opening in the home folder. Only the home folder
// is searched, since reading all of / would keep the computer busy for long
// and find the system's own files.
func defaultSources(home string) []source {
	return []source{{Name: "Computer", Path: "/", Indexed: true, Include: home}}
}

// formerMounts are the folders that older releases made sources beside the
// home folder.
var formerMounts = []string{"/media", "/run/media", "/mnt"}

// outdated reports whether list is what an older release wrote on its first
// start, the home folder and the folders other disks are mounted below, and
// nobody changed since.
func outdated(list []storedSource) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return wroteBefore(list, home)
}

func wroteBefore(list []storedSource, home string) bool {
	untouched := func(s storedSource, indexed bool) bool {
		return s.Name == filepath.Base(s.Path) && !s.ReadOnly && s.Indexed == indexed && s.SkipHidden == indexed && len(s.Exclude) == 0
	}
	if len(list) == 0 || list[0].Path != home || !untouched(list[0], true) {
		return false
	}
	for _, s := range list[1:] {
		if !slices.Contains(formerMounts, s.Path) || !untouched(s, false) {
			return false
		}
	}
	return true
}
