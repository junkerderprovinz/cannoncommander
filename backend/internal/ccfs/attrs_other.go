//go:build !windows

package ccfs

import "io/fs"

func attrsOf(fs.DirEntry) uint32 { return 0 }
