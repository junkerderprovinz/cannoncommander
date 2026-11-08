package main

// defaultSources are the home folder and /Volumes, where every other disk of
// a Mac shows up. The start disk's entry there is a link back to /.
func defaultSources(home string) []source {
	return homeAndMounts(home, []string{"/Volumes"}, isDir)
}
