//go:build !windows

package testutil

import "testing"

// Hide hides p the way the system does. A test tree names hidden entries
// with a dot, and outside Windows that is all it takes.
func Hide(testing.TB, string) {}
