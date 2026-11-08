//go:build !linux && !windows

package elevate

import (
	"context"
	"io"
)

func launchHelper(context.Context) (io.ReadWriteCloser, []byte, error) {
	return nil, nil, ErrUnsupported
}
