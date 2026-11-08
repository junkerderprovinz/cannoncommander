package main

import (
	"golang.org/x/sys/windows"
)

// defaultSources are the drives. The home folder lies on one of them, so it is
// where the panels open rather than a source of its own.
func defaultSources(string) []source {
	return drives(windows.GetLogicalDrives, isDir)
}

// drives turns the bit mask of drive letters into sources, leaving out a
// drive without a disk in it, such as an empty card reader.
func drives(mask func() (uint32, error), isDir func(string) bool) []source {
	bits, err := mask()
	if err != nil {
		return nil
	}
	var list []source
	for i := range 26 {
		if bits&(1<<i) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		if root := letter + `:\`; isDir(root) {
			list = append(list, source{Name: letter + ":", Path: root})
		}
	}
	return list
}
