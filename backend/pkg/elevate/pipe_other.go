//go:build !windows

package elevate

import "io"

func dialPipe(string) (io.ReadWriteCloser, error) { return nil, ErrUnsupported }
