//go:build !unix

package unraid

import "io/fs"

// inodeOf has nothing to report where there are no inodes. Unraid runs on
// Linux, other systems only build the package for development.
func inodeOf(fs.FileInfo) inode { return inode{} }

func ownerOf(fs.FileInfo) (uid, gid int, ok bool) { return 0, 0, false }
