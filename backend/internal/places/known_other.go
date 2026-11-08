//go:build desktop && !windows && !linux

package places

import "path/filepath"

// knownFolders are the folders a Mac keeps in every home folder.
func knownFolders(home string) []folder {
	return []folder{
		{"home", home},
		{"desktop", filepath.Join(home, "Desktop")},
		{"documents", filepath.Join(home, "Documents")},
		{"downloads", filepath.Join(home, "Downloads")},
		{"pictures", filepath.Join(home, "Pictures")},
		{"music", filepath.Join(home, "Music")},
		{"videos", filepath.Join(home, "Movies")},
	}
}
