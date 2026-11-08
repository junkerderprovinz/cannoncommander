package templates

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// find reads the templates folder that xdg-user-dirs names, ~/Templates or
// its translation such as ~/Vorlagen.
func find() []Template {
	if dir := templatesDir(); dir != "" {
		return fromFolder(dir, "xdg:")
	}
	return nil
}

func templatesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return filepath.Join(home, "Templates")
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		v, ok := strings.CutPrefix(strings.TrimSpace(lines.Text()), "XDG_TEMPLATES_DIR=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		v = strings.Replace(v, "$HOME", home, 1)
		// A templates folder set to the home folder itself means none.
		if filepath.Clean(v) == filepath.Clean(home) {
			return ""
		}
		return v
	}
	return filepath.Join(home, "Templates")
}
