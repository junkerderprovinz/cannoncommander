//go:build desktop

package places

import "golang.org/x/sys/windows"

var knownIDs = []struct {
	kind string
	id   *windows.KNOWNFOLDERID
}{
	{"desktop", windows.FOLDERID_Desktop},
	{"documents", windows.FOLDERID_Documents},
	{"downloads", windows.FOLDERID_Downloads},
	{"pictures", windows.FOLDERID_Pictures},
	{"music", windows.FOLDERID_Music},
	{"videos", windows.FOLDERID_Videos},
}

// knownFolders asks Windows where the folders are, which follows them when
// they were moved to another drive or into OneDrive.
func knownFolders(home string) []folder {
	out := []folder{{"home", home}}
	for _, k := range knownIDs {
		if p, err := windows.KnownFolderPath(k.id, 0); err == nil {
			out = append(out, folder{k.kind, p})
		}
	}
	return out
}
