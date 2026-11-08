//go:build !windows

package indexing

import "testing"

// hide does nothing outside Windows, where the dot in front of a name hides
// it.
func hide(*testing.T, string, bool) {}

func setAttrs(*testing.T, string, uint32) {}
