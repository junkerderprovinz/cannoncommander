package ccfs

import (
	"io/fs"
	"runtime"
	"strings"
)

// HiddenByName reports whether the name alone hides an entry. Explorer shows
// dot files and hides by the attribute only, so on Windows no name does.
func HiddenByName(name string) bool {
	return runtime.GOOS != "windows" && strings.HasPrefix(name, ".")
}

// The attributes of Windows that decide what is hidden, with the values of
// winnt.h.
const (
	attrHidden = 0x2
	attrSystem = 0x4
)

// HiddenEntry reports whether the file manager of the system hides d: on
// Windows its attribute decides, elsewhere the dot in front of its name.
func HiddenEntry(d fs.DirEntry) bool {
	return HiddenByName(d.Name()) || attrsOf(d)&attrHidden != 0
}

// ProtectedEntry reports a protected file of Windows, hidden and a system file
// at once, which Explorer keeps back even when it shows hidden files.
func ProtectedEntry(d fs.DirEntry) bool {
	return attrsOf(d)&(attrHidden|attrSystem) == attrHidden|attrSystem
}
