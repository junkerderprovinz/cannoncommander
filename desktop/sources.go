package main

import (
	"os"
	"path/filepath"
	"strings"
)

// homeAndMounts is the home folder, searched, and beside it each of the
// folders where the system mounts other disks that exists. A mount folder
// that holds the home is left out, since the server refuses two sources
// inside each other.
func homeAndMounts(home string, mounts []string, isDir func(string) bool) []source {
	list := []source{{Name: filepath.Base(home), Path: home, Indexed: true}}
	for _, m := range mounts {
		if !isDir(m) || within(home, m) || within(m, home) {
			continue
		}
		list = append(list, source{Name: filepath.Base(m), Path: m})
	}
	return list
}

// within reports whether p is root or lies below it.
func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
