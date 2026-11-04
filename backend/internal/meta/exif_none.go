//go:build 386 || arm

package meta

import "io"

// The EXIF library does not build for 32-bit targets, so pictures there
// only get what the image header tells.
func readExif(io.ReadSeeker, string) photo { return photo{} }
