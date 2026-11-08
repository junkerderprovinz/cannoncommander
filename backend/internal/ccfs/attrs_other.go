//go:build !windows

package ccfs

import "io/fs"

func attrsOf(fs.DirEntry) uint32 { return 0 }

func infoAttrs(fs.FileInfo) uint32 { return 0 }

// InCloud reports whether reading the file at p would download it first.
// Only Windows keeps such placeholders.
func InCloud(string) bool { return false }
