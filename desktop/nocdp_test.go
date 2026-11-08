//go:build !cdp

package main

import "testing"

func TestAReleaseBuildNeverOpensADebugPort(t *testing.T) {
	t.Setenv("CC_CDP_PORT", "9334")
	if got := debugArgs(); got != nil {
		t.Fatalf("got %v", got)
	}
}
