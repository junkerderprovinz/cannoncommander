package places

import (
	"os"
	"path/filepath"
	"strings"
)

var xdgDirs = []struct{ key, kind, english string }{
	{"XDG_DESKTOP_DIR", "desktop", "Desktop"},
	{"XDG_DOCUMENTS_DIR", "documents", "Documents"},
	{"XDG_DOWNLOAD_DIR", "downloads", "Downloads"},
	{"XDG_PICTURES_DIR", "pictures", "Pictures"},
	{"XDG_MUSIC_DIR", "music", "Music"},
	{"XDG_VIDEOS_DIR", "videos", "Videos"},
}

// knownFolders reads the user's folders from user-dirs.dirs, where the
// desktop keeps them in the language it was set up in.
func knownFolders(home string) []folder {
	conf := os.Getenv("XDG_CONFIG_HOME")
	if conf == "" {
		conf = filepath.Join(home, ".config")
	}
	data, _ := os.ReadFile(filepath.Join(conf, "user-dirs.dirs"))
	return userDirs(string(data), home)
}

// userDirs reads a user-dirs.dirs file. A folder it sets to the home folder
// itself is switched off, and one it leaves out has its English name.
func userDirs(data, home string) []folder {
	set := map[string]string{}
	for line := range strings.Lines(data) {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		val = strings.Trim(val, `"`)
		if rest, ok := strings.CutPrefix(val, "$HOME"); ok {
			val = home + rest
		}
		if filepath.IsAbs(val) {
			set[key] = filepath.Clean(val)
		}
	}
	home = filepath.Clean(home)
	out := []folder{{"home", home}}
	for _, d := range xdgDirs {
		p, ok := set[d.key]
		if !ok {
			p = filepath.Join(home, d.english)
		}
		if p != home {
			out = append(out, folder{d.kind, p})
		}
	}
	return out
}
