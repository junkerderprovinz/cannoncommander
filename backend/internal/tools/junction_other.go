//go:build !windows

package tools

func linkWithoutRights(_, _ string, err error) error { return err }
