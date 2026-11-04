package fileops

import "io/fs"

// Windows only serves for development, so free space and owners are not
// worth the system calls there.
func freeSpace(string) (int64, bool) { return 0, false }

func device(fs.FileInfo) (uint64, bool) { return 0, false }

func chown(string, fs.FileInfo) {}
