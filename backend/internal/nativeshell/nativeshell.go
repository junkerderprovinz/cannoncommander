// Package nativeshell is what the desktop app borrows from the computer's
// own file manager: its context menu, its properties dialog, starting a
// program as administrator, and its icons and thumbnails. Where a system
// has no such thing, the function returns ErrUnsupported.
package nativeshell

import "errors"

var (
	// ErrUnsupported is returned for what this system does not offer.
	ErrUnsupported = errors.New("this system does not offer that")
	// ErrNoPicture is returned when the system has no icon or thumbnail for
	// a file.
	ErrNoPicture = errors.New("the system has no picture for this file")
)

// Entry is a row of the system's menu that another program put there, as
// the settings list it so a person can hide it.
type Entry struct {
	// Key names the row in every menu it shows up in: its verb where it has
	// one, its text otherwise.
	Key   string
	Label string
	// Icon is the row's picture as a data URL, or "".
	Icon string
}

// MenuOptions shapes the system's own menu.
type MenuOptions struct {
	// X and Y are where the menu opens, in pixels of the screen counted from
	// the top left of the window's inside.
	X, Y int
	// Extended adds the rows Explorer shows only while Shift is held.
	Extended bool
	// Hidden reports whether the person hid the row with this key.
	Hidden func(key string) bool
}

// Action is a row another program adds to the menu of files on a desktop
// that draws no menu of its own, such as a Dolphin service menu or a Nemo
// action.
type Action struct {
	Entry
	// Group is the submenu the row sits in, or "".
	Group string
}
