//go:build desktop

package clipapi

import (
	"path/filepath"
	"strings"
)

// sameVolume reports whether two paths lie on one drive, where Explorer
// moves what is dropped rather than copying it.
func sameVolume(a, b string) bool {
	return strings.EqualFold(filepath.VolumeName(a), filepath.VolumeName(b))
}
