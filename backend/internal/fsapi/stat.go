package fsapi

import (
	"io/fs"
	"time"
)

// fileStat is what a listing row needs from one stat call.
type fileStat struct {
	mode  fs.FileMode
	size  int64
	mtime time.Time
	// btime is zero where the file system does not record it.
	btime time.Time
	ino   uint64
	nlink uint64
	// ids reports whether uid and gid were read.
	ids      bool
	uid, gid uint32
	// attrs are the file attributes of Windows, zero elsewhere.
	attrs uint32
}

// The attributes of Windows that a listing reads, with the values of
// winnt.h, so the rules built on them can be tested anywhere.
const (
	attrHidden = 0x2
	attrSystem = 0x4
)

func (s fileStat) isDir() bool { return s.mode.IsDir() }
