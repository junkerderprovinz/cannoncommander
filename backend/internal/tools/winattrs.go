package tools

import (
	"net/http"
	"runtime"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// The attributes of Windows that the properties change, with the values of
// winnt.h. pinned stands for Explorer's "always keep on this device", and
// clearing it frees the space of a cloud file again.
const (
	attrReadOnly = 0x1
	attrHidden   = 0x2
	attrArchive  = 0x20
	attrPinned   = 0x80000
	attrUnpinned = 0x100000
)

var flagBits = map[string]uint32{"readOnly": attrReadOnly, "hidden": attrHidden, "archive": attrArchive, "pinned": attrPinned}

func checkFlags(flags map[string]bool) error {
	for k := range flags {
		if _, ok := flagBits[k]; !ok {
			return badRequest("there is no attribute " + k)
		}
	}
	if len(flags) > 0 && runtime.GOOS != "windows" {
		return v1.Coded(http.StatusBadRequest, "windowsOnly", "these attributes exist on Windows only")
	}
	return nil
}

// withFlags turns the attributes on and off. A file is either kept on the
// device or free to be dropped, never both.
func withFlags(attrs uint32, flags map[string]bool) uint32 {
	for k, on := range flags {
		bit := flagBits[k]
		if k == "pinned" {
			attrs &^= attrPinned | attrUnpinned
			if !on {
				bit = attrUnpinned
			}
			on = true
		}
		if on {
			attrs |= bit
		} else {
			attrs &^= bit
		}
	}
	return attrs
}
