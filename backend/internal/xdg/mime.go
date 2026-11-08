package xdg

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// MimeDB is the part of the shared MIME database that names types of files
// and their icons.
type MimeDB struct {
	globs   []glob
	aliases map[string]string
	parents map[string][]string
	icons   map[string]string
	generic map[string]string
}

type glob struct {
	pattern, mime string
	weight        int
	caseSensitive bool
}

// MimeDB reads the database from the data folders. Where two folders say
// different things, the one that comes first wins.
func (d Desktop) MimeDB() *MimeDB {
	db := &MimeDB{aliases: map[string]string{}, parents: map[string][]string{}, icons: map[string]string{}, generic: map[string]string{}}
	first := func(m map[string]string, k, v string) {
		if _, seen := m[k]; !seen {
			m[k] = v
		}
	}
	for _, data := range d.Data {
		dir := filepath.Join(data, "mime")
		eachLine(filepath.Join(dir, "globs2"), func(line string) {
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
		eachLine(filepath.Join(dir, "aliases"), func(line string) {
			if alias, real, ok := strings.Cut(line, " "); ok {
				first(db.aliases, alias, real)
			}
		})
		eachLine(filepath.Join(dir, "subclasses"), func(line string) {
			if child, parent, ok := strings.Cut(line, " "); ok && !slices.Contains(db.parents[child], parent) {
				db.parents[child] = append(db.parents[child], parent)
			}
		})
		eachLine(filepath.Join(dir, "icons"), func(line string) {
			if m, icon, ok := strings.Cut(line, ":"); ok {
				first(db.icons, m, icon)
			}
		})
		eachLine(filepath.Join(dir, "generic-icons"), func(line string) {
			if m, icon, ok := strings.Cut(line, ":"); ok {
				first(db.generic, m, icon)
			}
		})
	}
	return db
}

// TypeOf names the type of a file by its name, and when no glob knows the
// name, by its start: plain text where it reads as text.
func (db *MimeDB) TypeOf(file string) string {
	if m := db.TypeByName(filepath.Base(file)); m != "" {
		return m
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

// TypeByName names the type of a file after the globs of the database: the
// heaviest match, and of those the longest. It is "" when no glob matches.
func (db *MimeDB) TypeByName(name string) string {
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
	if best.mime == "" {
		return ""
	}
	return db.Canonical(best.mime)
}

// Canonical turns an alias such as application/x-pdf into the name the
// database knows the type by.
func (db *MimeDB) Canonical(m string) string {
	if real, ok := db.aliases[m]; ok {
		return real
	}
	return m
}

// Lineage is m followed by the types it is a kind of, nearest first. Every
// text is also plain text.
func (db *MimeDB) Lineage(m string) []string {
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

// IconNames are the names of the icons for files of type m, best first:
// the one the database gives, the type with a dash for its slash, the
// generic one the database gives and the generic one of its media type.
func (db *MimeDB) IconNames(m string) []string {
	m = db.Canonical(m)
	var out []string
	add := func(name string) {
		if name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	add(db.icons[m])
	add(strings.ReplaceAll(m, "/", "-"))
	add(db.generic[m])
	if media, _, ok := strings.Cut(m, "/"); ok {
		add(media + "-x-generic")
	}
	return out
}
