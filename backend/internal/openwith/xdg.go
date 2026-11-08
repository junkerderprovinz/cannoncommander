package openwith

import (
	"bufio"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// xdg is what a Linux desktop knows about programs and types of files, read
// the way the freedesktop.org specifications lay it out: desktop entries,
// mimeapps.list and the shared MIME database.
type xdg struct {
	// data are the data folders, $XDG_DATA_HOME first and then
	// $XDG_DATA_DIRS, and config the config folders in the same order.
	data, config []string
	// desktops are $XDG_CURRENT_DESKTOP in lower case. Each may keep a
	// mimeapps.list of its own that comes before the common one.
	desktops []string
	// home holds ~/.icons, where older programs put their icons.
	home string
	// langs are the forms of the language a name is looked up in, the
	// longest first, such as de_AT and de.
	langs []string
	look  lookFunc
}

// lookFunc finds a program by name the way exec.LookPath does.
type lookFunc func(name string) (string, error)

// xdgFrom reads the folders and the language from the environment.
func xdgFrom(getenv func(string) string, look lookFunc) xdg {
	home := getenv("HOME")
	or := func(name, fallback string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return fallback
	}
	list := func(s string) []string {
		return slices.DeleteFunc(strings.Split(s, ":"), func(d string) bool { return d == "" })
	}
	x := xdg{
		data:   append([]string{or("XDG_DATA_HOME", path.Join(home, ".local", "share"))}, list(or("XDG_DATA_DIRS", "/usr/local/share:/usr/share"))...),
		config: append([]string{or("XDG_CONFIG_HOME", path.Join(home, ".config"))}, list(or("XDG_CONFIG_DIRS", "/etc/xdg"))...),
		home:   home,
		look:   look,
	}
	for _, d := range list(strings.ReplaceAll(getenv("XDG_CURRENT_DESKTOP"), ";", ":")) {
		x.desktops = append(x.desktops, strings.ToLower(d))
	}
	lang := getenv("LC_ALL")
	if lang == "" {
		lang = or("LC_MESSAGES", getenv("LANG"))
	}
	lang, _, _ = strings.Cut(lang, ".")
	lang, _, _ = strings.Cut(lang, "@")
	if lang != "" && lang != "C" && lang != "POSIX" {
		x.langs = append(x.langs, lang)
		if short, _, ok := strings.Cut(lang, "_"); ok {
			x.langs = append(x.langs, short)
		}
	}
	return x
}

// entry is a desktop entry of a program.
type entry struct {
	id, file  string
	name      string
	icon      string
	exec      string
	mime      []string
	noDisplay bool
}

// entries reads every desktop entry of a program. Where two folders hold
// the same id, the one that comes first wins, also when it hides the entry.
func (x xdg) entries() map[string]entry {
	all := map[string]entry{}
	seen := map[string]bool{}
	for _, d := range x.data {
		root := filepath.Join(d, "applications")
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
			if e, ok := x.readEntry(p); ok {
				e.id = id
				all[id] = e
			}
			return nil
		})
	}
	return all
}

// readEntry reads one desktop entry and reports whether it is a program
// that is there to run.
func (x xdg) readEntry(file string) (entry, bool) {
	f, err := os.Open(file)
	if err != nil {
		return entry{}, false
	}
	defer f.Close()
	e := entry{file: file}
	keys := map[string]string{}
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			in = line == "[Desktop Entry]"
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if in && ok {
			keys[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	if keys["Type"] != "Application" || keys["Hidden"] == "true" || keys["Exec"] == "" {
		return entry{}, false
	}
	if try := keys["TryExec"]; try != "" {
		if _, err := x.look(unescape(try)); err != nil {
			return entry{}, false
		}
	}
	e.name = unescape(keys["Name"])
	for _, l := range x.langs {
		if n, ok := keys["Name["+l+"]"]; ok {
			e.name = unescape(n)
			break
		}
	}
	e.icon = unescape(keys["Icon"])
	e.exec = unescape(keys["Exec"])
	e.noDisplay = keys["NoDisplay"] == "true"
	for _, m := range strings.Split(keys["MimeType"], ";") {
		if m = strings.TrimSpace(m); m != "" {
			e.mime = append(e.mime, m)
		}
	}
	return e, e.name != ""
}

// unescape undoes the escapes of a string value in a desktop entry.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i == len(s)-1 {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// mimeDB is the part of the shared MIME database that names types.
type mimeDB struct {
	globs   []glob
	aliases map[string]string
	parents map[string][]string
}

type glob struct {
	pattern, mime string
	weight        int
	caseSensitive bool
}

func (x xdg) mimeDB() mimeDB {
	db := mimeDB{aliases: map[string]string{}, parents: map[string][]string{}}
	for _, d := range x.data {
		eachLine(filepath.Join(d, "mime", "globs2"), func(line string) {
			parts := strings.Split(line, ":")
			if len(parts) < 3 {
				return
			}
			w, err := strconv.Atoi(parts[0])
			if err != nil {
				return
			}
			cs := len(parts) > 3 && slices.Contains(strings.Split(parts[3], ","), "cs")
			db.globs = append(db.globs, glob{pattern: parts[2], mime: parts[1], weight: w, caseSensitive: cs})
		})
		eachLine(filepath.Join(d, "mime", "aliases"), func(line string) {
			if alias, real, ok := strings.Cut(line, " "); ok {
				if _, seen := db.aliases[alias]; !seen {
					db.aliases[alias] = real
				}
			}
		})
		eachLine(filepath.Join(d, "mime", "subclasses"), func(line string) {
			if child, parent, ok := strings.Cut(line, " "); ok && !slices.Contains(db.parents[child], parent) {
				db.parents[child] = append(db.parents[child], parent)
			}
		})
	}
	return db
}

// mimeType names the type of file by its name, after the globs of the
// database: the heaviest match, and of those the longest. A name no glob
// matches is plain text when the start of the file reads as text.
func (db mimeDB) mimeType(file string) string {
	name := filepath.Base(file)
	var best glob
	for _, g := range db.globs {
		pattern, subject := g.pattern, name
		if !g.caseSensitive {
			pattern, subject = strings.ToLower(pattern), strings.ToLower(name)
		}
		if ok, _ := path.Match(pattern, subject); !ok {
			continue
		}
		if best.mime == "" || g.weight > best.weight || g.weight == best.weight && len(g.pattern) > len(best.pattern) {
			best = g
		}
	}
	if best.mime != "" {
		return db.canonical(best.mime)
	}
	f, err := os.Open(file)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	if strings.HasPrefix(http.DetectContentType(head[:n]), "text/plain") {
		return "text/plain"
	}
	return "application/octet-stream"
}

// canonical turns an alias such as application/x-pdf into the name the
// database knows the type by.
func (db mimeDB) canonical(m string) string {
	if real, ok := db.aliases[m]; ok {
		return real
	}
	return m
}

// lineage is m followed by the types it is a kind of, nearest first. Every
// text is also plain text.
func (db mimeDB) lineage(m string) []string {
	out := []string{m}
	for i := 0; i < len(out); i++ {
		for _, p := range db.parents[out[i]] {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	if strings.HasPrefix(m, "text/") && !slices.Contains(out, "text/plain") {
		out = append(out, "text/plain")
	}
	return out
}

func eachLine(file string, fn func(line string)) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" && line[0] != '#' {
			fn(line)
		}
	}
}

// associations are what the mimeapps.list files say about one type.
type associations struct {
	def     []string
	added   []string
	removed []string
}

// mimeapps reads what the lists say about m, in the order the
// specification gives them precedence.
func (x xdg) mimeapps(db mimeDB, m string) []associations {
	var files []string
	lists := func(dir string) {
		for _, d := range x.desktops {
			files = append(files, filepath.Join(dir, d+"-mimeapps.list"))
		}
		files = append(files, filepath.Join(dir, "mimeapps.list"))
	}
	for _, d := range x.config {
		lists(d)
	}
	for _, d := range x.data {
		lists(filepath.Join(d, "applications"))
		files = append(files, filepath.Join(d, "applications", "defaults.list"))
	}
	var out []associations
	for _, f := range files {
		var a associations
		group := ""
		eachLine(f, func(line string) {
			if line[0] == '[' {
				group = line
				return
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok || db.canonical(strings.TrimSpace(k)) != m {
				return
			}
			ids := slices.DeleteFunc(strings.Split(strings.TrimSpace(v), ";"), func(s string) bool { return s == "" })
			switch group {
			case "[Default Applications]":
				a.def = append(a.def, ids...)
			case "[Added Associations]":
				a.added = append(a.added, ids...)
			case "[Removed Associations]":
				a.removed = append(a.removed, ids...)
			}
		})
		out = append(out, a)
	}
	return out
}

// apps lists the programs for file the way GNOME and KDE order them: the
// default, the ones a person added, then every program that names the type,
// and after them the ones for the types it is a kind of.
func (x xdg) apps(file string) []app {
	all := x.entries()
	db := x.mimeDB()
	var def string
	var ids []string
	for _, m := range db.lineage(db.mimeType(file)) {
		removed := map[string]bool{}
		for _, a := range x.mimeapps(db, m) {
			for _, id := range a.removed {
				removed[id] = true
			}
			for _, id := range a.def {
				if _, ok := all[id]; ok && def == "" && !removed[id] {
					def = id
				}
			}
			for _, id := range a.added {
				if !removed[id] {
					ids = append(ids, id)
				}
			}
		}
		var named []entry
		for _, e := range all {
			if !removed[e.id] && slices.ContainsFunc(e.mime, func(t string) bool { return db.canonical(t) == m }) {
				named = append(named, e)
			}
		}
		slices.SortFunc(named, byName)
		for _, e := range named {
			ids = append(ids, e.id)
		}
	}
	if def != "" {
		ids = append([]string{def}, ids...)
	}
	var out []app
	have := map[string]bool{}
	for _, id := range ids {
		e, ok := all[id]
		if !ok || have[id] {
			continue
		}
		have[id] = true
		out = append(out, app{id: id, name: e.name, icon: e.icon, def: id == def})
	}
	return out
}

// every lists the programs a desktop shows in its menus, by name.
func (x xdg) every() []app {
	var shown []entry
	for _, e := range x.entries() {
		if !e.noDisplay {
			shown = append(shown, e)
		}
	}
	slices.SortFunc(shown, byName)
	out := make([]app, 0, len(shown))
	for _, e := range shown {
		out = append(out, app{id: e.id, name: e.name, icon: e.icon})
	}
	return out
}

func byName(a, b entry) int {
	if c := strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name)); c != 0 {
		return c
	}
	return strings.Compare(a.id, b.id)
}

// iconSizes are the folders of an icon theme looked in, the size of a menu
// icon first, then larger ones that scale down well.
var iconSizes = []string{"32x32", "48x48", "64x64", "scalable", "128x128", "256x256", "512x512", "24x24", "22x22", "16x16"}

// iconFile finds the file of the icon a desktop entry names: hicolor, the
// theme every program installs its icon into, then the other themes and
// the old pixmaps folder.
func (x xdg) iconFile(name string) (string, error) {
	exists := func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && info.Mode().IsRegular()
	}
	if filepath.IsAbs(name) {
		if exists(name) {
			return name, nil
		}
		return "", errNoIcon
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", errNoIcon
	}
	bases := []string{filepath.Join(x.home, ".icons")}
	for _, d := range x.data {
		bases = append(bases, filepath.Join(d, "icons"))
	}
	themes := []string{"hicolor"}
	for _, b := range bases {
		dirs, _ := os.ReadDir(b)
		for _, d := range dirs {
			if d.IsDir() && !slices.Contains(themes, d.Name()) {
				themes = append(themes, d.Name())
			}
		}
	}
	for _, theme := range themes {
		for _, b := range bases {
			for _, size := range iconSizes {
				for _, ext := range []string{".png", ".svg"} {
					if p := filepath.Join(b, theme, size, "apps", name+ext); exists(p) {
						return p, nil
					}
				}
			}
		}
	}
	for _, d := range x.data {
		for _, ext := range []string{".png", ".svg"} {
			if p := filepath.Join(d, "pixmaps", name+ext); exists(p) {
				return p, nil
			}
		}
	}
	return "", errNoIcon
}

var errNoExec = errors.New("the desktop entry has no command to run")

// command is how a desktop entry's Exec line runs the program on file: the
// line split into its arguments and the field codes filled in. A line
// without a code for files gets the file at its end.
func command(e entry, file string) ([]string, error) {
	words, err := splitExec(e.exec)
	if err != nil {
		return nil, err
	}
	var out []string
	took := false
	for _, w := range words {
		switch w {
		case "%f", "%F", "%u", "%U":
			out = append(out, file)
			took = true
			continue
		case "%i":
			if e.icon != "" {
				out = append(out, "--icon", e.icon)
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
			case 'f', 'F', 'u', 'U':
				b.WriteString(file)
				took = true
			case 'c':
				b.WriteString(e.name)
			case 'k':
				b.WriteString(e.file)
			}
		}
		if b.Len() > 0 || !strings.HasPrefix(w, "%") {
			out = append(out, b.String())
		}
	}
	if len(out) == 0 {
		return nil, errNoExec
	}
	if !took {
		out = append(out, file)
	}
	return out, nil
}

// splitExec splits an Exec line into its arguments. Double quotes hold an
// argument with spaces, and a backslash in them keeps the next character.
func splitExec(s string) ([]string, error) {
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
