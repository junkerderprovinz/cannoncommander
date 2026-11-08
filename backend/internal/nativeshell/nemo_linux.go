package nativeshell

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// nemo lists the Nemo actions that fit sel. Where two folders hold an
// action of the same name, the one that comes first wins.
func (k *desk) nemo(sel selection) []action {
	var out []action
	seen := map[string]bool{}
	for _, data := range k.Data {
		dir := filepath.Join(data, "nemo", "actions")
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			name := f.Name()
			if seen[name] || !strings.HasSuffix(name, ".nemo_action") {
				continue
			}
			seen[name] = true
			kf, err := xdg.ReadKeyFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			g := kf["Nemo Action"]
			label, command := k.Localized(g, "Name"), g.String("Exec")
			if g["Active"] == "false" || label == "" || command == "" || !k.nemoFits(g, sel) {
				continue
			}
			// An Exec line in angle brackets names a program next to the
			// action file.
			if inner, ok := strings.CutPrefix(command, "<"); ok {
				command = shellQuote(dir) + "/" + strings.TrimSuffix(inner, ">")
			}
			first := filepath.Base(sel.items[0].path)
			label = strings.NewReplacer("%N", first, "%f", first, "%%", "%").Replace(label)
			out = append(out, action{
				Action: Action{Entry: Entry{Key: "nemo:" + name, Label: label}},
				icon:   g.String("Icon-Name"),
				launches: func(sel selection) ([]launch, error) {
					args, err := nemoCommand(command, g, sel)
					if err != nil {
						return nil, err
					}
					return []launch{{args: args, dir: sel.dir}}, nil
				},
			})
		}
	}
	return out
}

// nemoFits reports whether a Nemo action is for sel by its selection,
// extensions, types and dependencies. None of Nemo's conditions can be
// checked cheaply from here, so an action with any is left out.
func (k *desk) nemoFits(g xdg.Group, sel selection) bool {
	n := len(sel.items)
	switch s := strings.ToLower(g["Selection"]); s {
	case "s", "":
		if n != 1 {
			return false
		}
	case "m":
		if n < 2 {
			return false
		}
	case "any", "notnone":
	default:
		if want, err := strconv.Atoi(s); err != nil || n != want {
			return false
		}
	}
	if len(g.List("Conditions")) > 0 {
		return false
	}
	exts, types := g.List("Extensions"), g.List("Mimetypes")
	for _, it := range sel.items {
		if !slices.ContainsFunc(exts, func(e string) bool { return nemoExtension(it, e) }) &&
			!slices.ContainsFunc(types, func(t string) bool { return k.is(it, t) }) {
			return false
		}
	}
	for _, dep := range g.List("Dependencies") {
		program, absent := strings.CutPrefix(dep, "!")
		if k.has(program) == absent {
			return false
		}
	}
	return true
}

// has reports whether a program is there, by its path or by its name.
func (k *desk) has(program string) bool {
	if filepath.IsAbs(program) {
		_, err := os.Stat(program)
		return err == nil
	}
	_, err := k.Look(program)
	return err == nil
}

func nemoExtension(it item, ext string) bool {
	name := strings.ToLower(filepath.Base(it.path))
	switch ext = strings.ToLower(ext); ext {
	case "any":
		return true
	case "dir":
		return it.dir
	case "nodirs":
		return !it.dir
	case "none":
		return !it.dir && !strings.Contains(name, ".")
	}
	return strings.HasSuffix(name, ext)
}

// nemoCommand fills in the codes of a Nemo action's Exec line and splits
// it the way Nemo does.
func nemoCommand(command string, g xdg.Group, sel selection) ([]string, error) {
	quote := shellQuote
	if strings.EqualFold(g["Quote"], "double") {
		quote = doubleQuote
	}
	sep := g.String("Separator")
	if sep == "" {
		sep = " "
	}
	each := func(values []string) string {
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = quote(v)
		}
		return strings.Join(quoted, sep)
	}
	first := filepath.Base(sel.items[0].path)
	return splitArgv(fill(command, map[byte]string{
		'U': each(sel.uris()),
		'F': each(sel.paths()),
		'P': quote(sel.dir),
		'p': quote(filepath.Base(sel.dir)),
		'f': quote(first),
		'N': quote(first),
		'e': quote(strings.TrimSuffix(first, filepath.Ext(first))),
		'D': "",
		'X': "",
	}))
}
