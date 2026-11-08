//go:build cdp

package main

import (
	"slices"
	"testing"
)

func TestATestBuildOpensTheDebugPortItIsGiven(t *testing.T) {
	for port, want := range map[string][]string{
		"9334":   {"--remote-debugging-port=9334"},
		"":       nil,
		"0":      nil,
		"70000":  nil,
		"9334; ": nil,
	} {
		t.Setenv("CC_CDP_PORT", port)
		if got := debugArgs(); !slices.Equal(got, want) {
			t.Errorf("CC_CDP_PORT=%q: got %v", port, got)
		}
	}
}
