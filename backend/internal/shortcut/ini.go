package shortcut

import (
	"net/url"
	"strings"
)

// section returns the keys of one section of an INI file, which both the
// .url files of Windows and the .desktop launchers are. Later sections such
// as the actions of a launcher stay out.
func section(data []byte, name string) (map[string]string, bool) {
	keys := map[string]string{}
	in, found := false, false
	for _, line := range strings.Split(strings.TrimPrefix(string(data), string(rune(0xFEFF))), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			in = strings.EqualFold(line, "["+name+"]")
			found = found || in
			continue
		}
		if k, v, ok := strings.Cut(line, "="); in && ok {
			k = strings.TrimSpace(k)
			if _, seen := keys[k]; !seen {
				keys[k] = strings.TrimSpace(v)
			}
		}
	}
	return keys, found
}

func parseURL(data []byte) (Shortcut, error) {
	keys, ok := section(data, "InternetShortcut")
	if !ok || keys["URL"] == "" {
		return Shortcut{}, errNotShortcut
	}
	return Shortcut{Kind: URL, Target: keys["URL"], Icon: keys["IconFile"]}, nil
}

// parseDesktop reads a launcher of the freedesktop.org specification. A
// link to a file:// address leads to a path, as Nautilus opens it.
func parseDesktop(data []byte) (Shortcut, error) {
	keys, ok := section(data, "Desktop Entry")
	if !ok {
		return Shortcut{}, errNotShortcut
	}
	s := Shortcut{Kind: Desktop, Name: keys["Name"], Icon: keys["Icon"]}
	switch keys["Type"] {
	case "Link":
		s.Target = keys["URL"]
		if u, err := url.Parse(s.Target); err == nil && u.Scheme == "file" {
			s.Target = u.Path
		}
	case "Application":
		s.Target = keys["Exec"]
	}
	return s, nil
}
