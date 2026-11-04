package ccfs

import (
	"errors"
	"syscall"
)

// errorNotSameDevice is ERROR_NOT_SAME_DEVICE, the Windows answer to a
// rename across drives.
const errorNotSameDevice = syscall.Errno(17)

// CrossDevice reports whether a rename failed because source and target
// lie on different file systems, so the caller has to copy instead.
func CrossDevice(err error) bool {
	return errors.Is(err, syscall.EXDEV) || errors.Is(err, errorNotSameDevice)
}
