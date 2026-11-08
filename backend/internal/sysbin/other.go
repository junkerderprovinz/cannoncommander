//go:build !linux && !windows

package sysbin

// Open reports that this system's trash is not supported.
func Open() (Bin, error) { return nil, ErrUnsupported }
