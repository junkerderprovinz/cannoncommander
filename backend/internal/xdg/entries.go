package xdg

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// Entry is a desktop entry of a program.
type Entry struct {
	// ID names the entry: its path below an applications folder, with
	// dashes for the slashes.
	ID, File  string
	Name      string
	Icon      string
	Exec      string
	Mime      []string
	NoDisplay bool
}

// Entries reads every desktop entry of a program. Where two folders hold
// the same id, the one that comes first wins, also when it hides the entry.
func (d Desktop) Entries() map[string]Entry {
	all := map[string]Entry{}
	seen := map[string]bool{}
	for _, data := range d.Data {
		root := filepath.Join(data, "applications")
		_ = filepath.WalkDir(root, func(p string, de fs.DirEntry, err error) error {
			if err != nil || de.IsDir() || !strings.HasSuffix(p, ".desktop") {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return nil
			}
			id := strings.ReplaceAll(filepath.ToSlash(rel), "/", "-")
			if seen[id] {
				return nil
			}
			seen[id] = true
			if e, ok := d.ReadEntry(p); ok {
				e.ID = id
				all[id] = e
			}
			return nil
		})
	}
	return all
}

// ReadEntry reads one desktop entry and reports whether it is a program
// that is there to run.
func (d Desktop) ReadEntry(file string) (Entry, bool) {
	kf, err := ReadKeyFile(file)
	if err != nil {
		return Entry{}, false
	}
	g := kf["Desktop Entry"]
	if g["Type"] != "Application" || g.Bool("Hidden") || g["Exec"] == "" {
		return Entry{}, false
	}
	if try := g.String("TryExec"); try != "" {
		if _, err := d.Look(try); err != nil {
			return Entry{}, false
		}
	}
	e := Entry{
		File:      file,
		Name:      d.Localized(g, "Name"),
		Icon:      g.String("Icon"),
		Exec:      g.String("Exec"),
		Mime:      g.List("MimeType"),
		NoDisplay: g.Bool("NoDisplay"),
	}
	return e, e.Name != ""
}

// Associations are what one mimeapps.list says about a type.
type Associations struct {
	Default []string
	Added   []string
	Removed []string
}

// MimeApps reads what the lists say about m, in the order the
// specification gives them precedence.
func (d Desktop) MimeApps(db *MimeDB, m string) []Associations {
	var files []string
	lists := func(dir string) {
		for _, name := range d.Desktops {
			files = append(files, filepath.Join(dir, name+"-mimeapps.list"))
		}
		files = append(files, filepath.Join(dir, "mimeapps.list"))
	}
	for _, c := range d.Config {
		lists(c)
	}
	for _, data := range d.Data {
		lists(filepath.Join(data, "applications"))
		files = append(files, filepath.Join(data, "applications", "defaults.list"))
	}
	var out []Associations
	for _, f := range files {
		var a Associations
		group := ""
		eachLine(f, func(line string) {
			if line[0] == '[' {
				group = line
				return
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok || db.Canonical(strings.TrimSpace(k)) != m {
				return
			}
			ids := slices.DeleteFunc(strings.Split(strings.TrimSpace(v), ";"), func(s string) bool { return s == "" })
			switch group {
			case "[Default Applications]":
				a.Default = append(a.Default, ids...)
			case "[Added Associations]":
				a.Added = append(a.Added, ids...)
			case "[Removed Associations]":
				a.Removed = append(a.Removed, ids...)
			}
		})
		out = append(out, a)
	}
	return out
}

// ErrNoExec is returned for a desktop entry whose Exec line runs nothing.
var ErrNoExec = errors.New("the desktop entry has no command to run")

// Expand splits e's Exec line into the commands that run the program on
// files, one for each file where the line takes a single one. A line
// without a code for files gets them at its end.
func Expand(e Entry, files []string) ([][]string, error) {
	words, err := SplitExec(e.Exec)
	if err != nil {
		return nil, err
	}
	list := slices.ContainsFunc(words, func(w string) bool { return strings.Contains(w, "%F") || strings.Contains(w, "%U") })
	single := slices.ContainsFunc(words, func(w string) bool { return strings.Contains(w, "%f") || strings.Contains(w, "%u") })
	if single && !list && len(files) > 1 {
		var out [][]string
		for _, f := range files {
			args, err := expand(e, words, []string{f})
			if err != nil {
				return nil, err
			}
			out = append(out, args)
		}
		return out, nil
	}
	args, err := expand(e, words, files)
	if err != nil {
		return nil, err
	}
	return [][]string{args}, nil
}

func expand(e Entry, words, files []string) ([]string, error) {
	first := ""
	if len(files) > 0 {
		first = files[0]
	}
	var out []string
	took := false
	for _, w := range words {
		switch w {
		case "%f", "%u":
			if first != "" {
				out = append(out, first)
			}
			took = true
			continue
		case "%F", "%U":
			out = append(out, files...)
			took = true
			continue
		case "%i":
			if e.Icon != "" {
				out = append(out, "--icon", e.Icon)
			}
			continue
		}
		var b strings.Builder
		for i := 0; i < len(w); i++ {
			if w[i] != '%' || i == len(w)-1 {
				b.WriteByte(w[i])
				continue
			}
			i++
			switch w[i] {
			case '%':
				b.WriteByte('%')
			case 'f', 'u':
				b.WriteString(first)
				took = true
			case 'F', 'U':
				b.WriteString(strings.Join(files, " "))
				took = true
			case 'c':
				b.WriteString(e.Name)
			case 'k':
				b.WriteString(e.File)
			}
		}
		if b.Len() > 0 || !strings.HasPrefix(w, "%") {
			out = append(out, b.String())
		}
	}
	if len(out) == 0 {
		return nil, ErrNoExec
	}
	if !took {
		out = append(out, files...)
	}
	return out, nil
}

// SplitExec splits an Exec line into its arguments. Double quotes hold an
// argument with spaces, and a backslash in them keeps the next character.
func SplitExec(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	quoted, inWord := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quoted && c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == '"':
			quoted = !quoted
			inWord = true
		case !quoted && (c == ' ' || c == '\t'):
			if inWord {
				out = append(out, b.String())
				b.Reset()
				inWord = false
			}
		default:
			b.WriteByte(c)
			inWord = true
		}
	}
	if quoted {
		return nil, errors.New("the Exec line leaves a quote open")
	}
	if inWord {
		out = append(out, b.String())
	}
	return out, nil
}
