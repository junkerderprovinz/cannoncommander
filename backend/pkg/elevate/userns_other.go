//go:build !linux

package elevate

func userNamespaces(bool) error { return ErrUnsupported }
