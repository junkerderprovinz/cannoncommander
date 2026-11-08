package main

// defaultSources are the home folder and the folders other disks are mounted
// below: /media and /run/media, where the desktop mounts a stick, and /mnt.
func defaultSources(home string) []source {
	return homeAndMounts(home, []string{"/media", "/run/media", "/mnt"}, isDir)
}
