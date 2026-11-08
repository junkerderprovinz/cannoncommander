//go:build !windows

package tools

import "errors"

func setFlags(string, map[string]bool) error { return errors.ErrUnsupported }
