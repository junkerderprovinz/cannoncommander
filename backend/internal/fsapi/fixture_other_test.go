//go:build !windows

package fsapi

import "testing"

func fixtureBase(t testing.TB) string { return t.TempDir() }
