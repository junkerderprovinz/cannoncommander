package nativeshell

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// nautilus lists the scripts in Nautilus's scripts folder. Nautilus offers
// each for any file, and a script in a subfolder in a submenu of that name.
func (k *desk) nautilus(selection) []action {
	root := filepath.Join(k.Data[0], "nautilus", "scripts")
	var out []action
	_ = filepath.WalkDir(root, func(p string, de fs.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		if strings.HasPrefix(de.Name(), ".") {
			if de.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if de.IsDir() || !executable(p) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		group := filepath.ToSlash(filepath.Dir(rel))
		if group == "." {
			group = ""
		}
		out = append(out, action{
			Action: Action{Entry: Entry{Key: "nautilus:" + filepath.ToSlash(rel), Label: de.Name()}, Group: group},
			launches: func(sel selection) ([]launch, error) {
				paths := sel.paths()
				env := []string{
					"NAUTILUS_SCRIPT_SELECTED_FILE_PATHS=" + lines(paths),
					"NAUTILUS_SCRIPT_SELECTED_URIS=" + lines(sel.uris()),
					"NAUTILUS_SCRIPT_CURRENT_URI=" + xdg.FileURI(sel.dir),
				}
				return []launch{{args: append([]string{p}, paths...), dir: sel.dir, env: env}}, nil
			},
		})
		return nil
	})
	return out
}

// lines ends each of values with a line break, as Nautilus lists files to
// its scripts.
func lines(values []string) string {
	var b strings.Builder
	for _, v := range values {
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.String()
}
