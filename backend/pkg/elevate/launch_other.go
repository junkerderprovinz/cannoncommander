//go:build !linux && !windows

package elevate

import "context"

func launchHelper(context.Context, int, []byte) (func() error, error) { return nil, ErrUnsupported }
