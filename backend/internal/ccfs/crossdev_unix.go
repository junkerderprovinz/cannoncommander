//go:build !windows

package ccfs

import (
	"errors"
	"syscall"
)

// CrossDevice reports whether a rename failed because source and target
// lie on different file systems, so the caller has to copy instead.
func CrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
