// Package shortcut reads the shortcut files of the desktops: .lnk and .url
// of Windows and the .desktop launchers of Linux, so a listing can say
// where they lead. It also writes the .lnk that Explorer's "Create
// shortcut" makes.
package shortcut

import (
	"errors"
	"io"
	"os"
	"strings"
)

// Kinds of shortcuts, named by their file ending.
const (
	Lnk     = "lnk"
	URL     = "url"
	Desktop = "desktop"
)

// Shortcut is where a shortcut file leads.
type Shortcut struct {
	Kind string `json:"kind"`
	// Target is a path on this computer, a network path or an address.
	Target string `json:"target,omitempty"`
	// Path is Target as a path of the API, when it lies on a source.
	Path string `json:"path,omitempty"`
	// Dir says that the target is a folder, as far as the file tells.
	Dir  bool   `json:"dir,omitempty"`
	Args string `json:"args,omitempty"`
	// Name is the name a .desktop launcher shows instead of its file name.
	Name string `json:"name,omitempty"`
	// Icon is where the icon comes from: a file with an index for a .lnk,
	// a name of the icon theme for a launcher.
	Icon string `json:"icon,omitempty"`
}

// maxSize bounds what a listing reads of a shortcut. Real ones stay far
// below it.
const maxSize = 64 << 10

var errNotShortcut = errors.New("not a shortcut file")

// KindOf returns the kind of shortcut a file name has, or "".
func KindOf(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot < 1 {
		return ""
	}
	switch k := strings.ToLower(name[dot+1:]); k {
	case Lnk, URL, Desktop:
		return k
	}
	return ""
}

// Read reads the shortcut file at p.
func Read(p string) (Shortcut, error) {
	kind := KindOf(p)
	if kind == "" {
		return Shortcut{}, errNotShortcut
	}
	f, err := os.Open(p)
	if err != nil {
		return Shortcut{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSize))
	if err != nil {
		return Shortcut{}, err
	}
	switch kind {
	case Lnk:
		return parseLnk(data)
	case URL:
		return parseURL(data)
	}
	return parseDesktop(data)
}
