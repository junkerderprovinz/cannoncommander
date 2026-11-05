package ccfs

import "io/fs"

const noFollow = 0

func singleLink(fs.FileInfo) bool { return true }
