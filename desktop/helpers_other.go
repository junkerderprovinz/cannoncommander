//go:build !windows

package main

import "errors"

// fetchElevated and updateElevated are never called: only Windows has a copy
// installed for all users.
func fetchElevated() error { return errors.ErrUnsupported }

func updateElevated() error { return errors.ErrUnsupported }
