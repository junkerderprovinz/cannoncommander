package fsapi

import "sync/atomic"

var remoteNamer atomic.Pointer[func(path string) string]

// SetRemotes lets the remotes package name the rclone remote mounted at a
// folder, such as "rclone: gdrive" for /mnt/remotes/gdrive. Listings and the tree
// mark those folders with it.
func SetRemotes(at func(path string) string) { remoteNamer.Store(&at) }

func remoteAt(p string) string {
	if f := remoteNamer.Load(); f != nil && *f != nil {
		return (*f)(p)
	}
	return ""
}
