//go:build !windows

package main

import "errors"

// fetchElevated is never called: only Windows has a copy installed for all
// users.
func fetchElevated() error { return errors.ErrUnsupported }
