//go:build !desktop

package fsapi

// withSystemThumbs returns own, as the container's server has no thumbnail
// handlers of a desktop to ask.
func withSystemThumbs(own thumbnailer) thumbnailer { return own }
