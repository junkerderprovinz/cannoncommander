// Package xdg reads what a Linux desktop knows about programs, types of
// files, icons and thumbnails, the way the freedesktop.org specifications
// lay it out.
package xdg

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Desktop is where a desktop keeps what it knows.
type Desktop struct {
	// Data are the data folders, $XDG_DATA_HOME first and then
	// $XDG_DATA_DIRS, and Config the config folders in the same order.
	Data, Config []string
	// Cache is $XDG_CACHE_HOME, which holds the thumbnails.
	Cache string
	// Desktops are $XDG_CURRENT_DESKTOP in lower case. Each may keep a
	// mimeapps.list of its own that comes before the common one.
	Desktops []string
	// Home holds ~/.icons, where older programs put their icons.
	Home string
	// Langs are the forms of the language a name is looked up in, the
	// longest first, such as de_AT and de.
	Langs []string
	// Look finds a program by name the way exec.LookPath does.
	Look func(name string) (string, error)
	// Output runs a program without a shell and returns what it prints, as
	// exec.Cmd.Output does. Where it is nil, no program is asked.
	Output func(program string, args ...string) ([]byte, error)
}

// Current is the desktop of this process's environment.
func Current() Desktop {
	d := FromEnv(os.Getenv, exec.LookPath)
	d.Output = output
	return d
}

// output gives up on a program after a few seconds, so one that hangs
// cannot hold up a menu. WaitDelay ends the wait for its output even when
// something it started keeps the pipe open.
func output(program string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.WaitDelay = time.Second
	return cmd.Output()
}

var errNoOutput = errors.New("no program may be asked")

// GSetting reads a key of GSettings with the gsettings program and returns
// it as GLib prints it, such as 'Adwaita', true or uint32 5.
func (d Desktop) GSetting(schema, key string) (string, error) {
	if d.Output == nil {
		return "", errNoOutput
	}
	program, err := d.Look("gsettings")
	if err != nil {
		return "", err
	}
	out, err := d.Output(program, "get", schema, key)
	return strings.TrimSpace(string(out)), err
}

// UnquoteGVariant reads a string in GLib's text format for values, in single
// or double quotes with backslash escapes.
func UnquoteGVariant(s string) (string, bool) {
	if len(s) < 2 || s[0] != s[len(s)-1] || s[0] != '\'' && s[0] != '"' {
		return "", false
	}
	var b strings.Builder
	for i := 1; i < len(s)-1; i++ {
		c := s[i]
		if c == s[0] {
			return "", false
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i++; i == len(s)-1 {
			return "", false
		}
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'u', 'U':
			n := 4
			if s[i] == 'U' {
				n = 8
			}
			if i+n >= len(s) {
				return "", false
			}
			r, err := strconv.ParseUint(s[i+1:i+1+n], 16, 32)
			if err != nil {
				return "", false
			}
			b.WriteRune(rune(r))
			i += n
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String(), true
}

// FromEnv reads the folders and the language from the environment.
func FromEnv(getenv func(string) string, look func(string) (string, error)) Desktop {
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
	d := Desktop{
		Data:   append([]string{or("XDG_DATA_HOME", path.Join(home, ".local", "share"))}, list(or("XDG_DATA_DIRS", "/usr/local/share:/usr/share"))...),
		Config: append([]string{or("XDG_CONFIG_HOME", path.Join(home, ".config"))}, list(or("XDG_CONFIG_DIRS", "/etc/xdg"))...),
		Cache:  or("XDG_CACHE_HOME", path.Join(home, ".cache")),
		Home:   home,
		Look:   look,
	}
	for _, name := range list(strings.ReplaceAll(getenv("XDG_CURRENT_DESKTOP"), ";", ":")) {
		d.Desktops = append(d.Desktops, strings.ToLower(name))
	}
	locale := getenv("LC_ALL")
	if locale == "" {
		locale = or("LC_MESSAGES", getenv("LANG"))
	}
	// gettext reads $LANGUAGE only when a locale other than C is set.
	if l := bareLang(locale); l != "" && l != "C" && l != "POSIX" {
		for _, l := range append(list(getenv("LANGUAGE")), l) {
			d.addLang(bareLang(l))
		}
	}
	return d
}

func bareLang(l string) string {
	l, _, _ = strings.Cut(l, ".")
	l, _, _ = strings.Cut(l, "@")
	return l
}

func (d *Desktop) addLang(l string) {
	if l == "" || l == "C" || l == "POSIX" {
		return
	}
	for _, form := range []string{l, strings.Split(l, "_")[0]} {
		if !slices.Contains(d.Langs, form) {
			d.Langs = append(d.Langs, form)
		}
	}
}

// KeyFile is a file in the format of desktop entries, groups such as
// [Desktop Entry] that hold keys. Values are kept as written, escapes and
// all.
type KeyFile map[string]Group

// Group is one group of a KeyFile.
type Group map[string]string

// ReadKeyFile reads a key file. A key that shows up twice keeps its last
// value.
func ReadKeyFile(file string) (KeyFile, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	kf := KeyFile{}
	var g Group
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			name := line[1 : len(line)-1]
			if g = kf[name]; g == nil {
				g = Group{}
				kf[name] = g
			}
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && g != nil {
			g[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return kf, sc.Err()
}

// String is the value of key with its escapes undone.
func (g Group) String(key string) string { return Unescape(g[key]) }

// Bool reports whether key is true.
func (g Group) Bool(key string) bool { return g[key] == "true" }

// List is the value of key split at its semicolons, without empty items.
func (g Group) List(key string) []string {
	var out []string
	s := g[key]
	start := 0
	for i := 0; i <= len(s); i++ {
		switch {
		case i < len(s) && s[i] == '\\':
			i++
		case i == len(s) || s[i] == ';':
			if item := strings.TrimSpace(Unescape(s[start:i])); item != "" {
				out = append(out, item)
			}
			start = i + 1
		}
	}
	return out
}

// Localized is the value of key in the user's language, such as
// Name[de_AT], or the plain one.
func (d Desktop) Localized(g Group, key string) string {
	for _, l := range d.Langs {
		if v, ok := g[key+"["+l+"]"]; ok {
			return Unescape(v)
		}
	}
	return g.String(key)
}

// Unescape undoes the escapes of a string value in a key file.
func Unescape(s string) string {
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
