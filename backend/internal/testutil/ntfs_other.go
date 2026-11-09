//go:build !windows

package testutil

import "testing"

// LocalDir is a fresh temporary folder. Outside Windows any of them keeps
// what the tests ask of a local disk.
func LocalDir(t testing.TB) string { return t.TempDir() }
