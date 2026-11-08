//go:build !windows

package fsapi

func cloudTagged(string, uint32) bool { return false }
