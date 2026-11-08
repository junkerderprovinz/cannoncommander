//go:build linux

package nativeshell

import "context"

func NativeMenu() bool { return false }

func ShowMenu([]string, MenuOptions) ([]Entry, error) { return nil, ErrUnsupported }

func Actions([]string) ([]Action, error) { return nil, nil }

func RunAction([]string, string) error { return ErrUnsupported }

func HasProperties() bool { return false }

func Properties([]string) error { return ErrUnsupported }

func CanRunAsAdmin(string) bool { return false }

func RunAsAdmin(string) error { return ErrUnsupported }

func Icon(string, int) ([]byte, string, error) { return nil, "", ErrUnsupported }

func TypeIcon(string, bool, int) ([]byte, string, error) { return nil, "", ErrUnsupported }

func Thumbnail(context.Context, string, int) ([]byte, error) { return nil, ErrUnsupported }
