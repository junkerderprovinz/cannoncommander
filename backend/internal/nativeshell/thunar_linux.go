package nativeshell

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ucaAction is a custom action in Thunar's uca.xml. A type of file is
// offered where its element is there, empty as it is.
type ucaAction struct {
	Icon     string    `xml:"icon"`
	Names    []ucaName `xml:"name"`
	Submenu  string    `xml:"submenu"`
	ID       string    `xml:"unique-id"`
	Command  string    `xml:"command"`
	Range    string    `xml:"range"`
	Patterns string    `xml:"patterns"`
	Dirs     *struct{} `xml:"directories"`
	Audio    *struct{} `xml:"audio-files"`
	Image    *struct{} `xml:"image-files"`
	Text     *struct{} `xml:"text-files"`
	Video    *struct{} `xml:"video-files"`
	Other    *struct{} `xml:"other-files"`
}

type ucaName struct {
	Lang string `xml:"lang,attr"`
	Text string `xml:",chardata"`
}

// thunar lists Thunar's custom actions that fit sel, from the first
// uca.xml in the config folders, as Thunar reads only that one.
func (k *desk) thunar(sel selection) []action {
	var actions struct {
		List []ucaAction `xml:"action"`
	}
	for _, c := range k.Config {
		if b, err := os.ReadFile(filepath.Join(c, "Thunar", "uca.xml")); err == nil {
			if xml.Unmarshal(b, &actions) != nil {
				return nil
			}
			break
		}
	}
	var out []action
	for _, a := range actions.List {
		label := k.ucaLabel(a.Names)
		if label == "" || strings.TrimSpace(a.Command) == "" || !k.ucaFits(a, sel) {
			continue
		}
		key := a.ID
		if key == "" {
			key = label
		}
		command := a.Command
		out = append(out, action{
			Action: Action{Entry: Entry{Key: "thunar:" + key, Label: label}, Group: strings.TrimSpace(a.Submenu)},
			icon:   strings.TrimSpace(a.Icon),
			launches: func(sel selection) ([]launch, error) {
				args, err := thunarCommand(command, sel)
				if err != nil {
					return nil, err
				}
				return []launch{{args: args, dir: sel.dir}}, nil
			},
		})
	}
	return out
}

func (k *desk) ucaLabel(names []ucaName) string {
	for _, l := range k.Langs {
		for _, n := range names {
			if n.Lang == l {
				return strings.TrimSpace(n.Text)
			}
		}
	}
	for _, n := range names {
		if n.Lang == "" {
			return strings.TrimSpace(n.Text)
		}
	}
	return ""
}

// ucaFits reports whether a custom action is for sel: a count within its
// range, and each file matching one of its patterns and of a type it is
// for.
func (k *desk) ucaFits(a ucaAction, sel selection) bool {
	if !inRange(strings.TrimSpace(a.Range), len(sel.items)) {
		return false
	}
	var patterns []string
	for _, p := range strings.Split(a.Patterns, ";") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, strings.ToLower(p))
		}
	}
	if len(patterns) == 0 {
		patterns = []string{"*"}
	}
	for _, it := range sel.items {
		name := strings.ToLower(filepath.Base(it.path))
		if !slices.ContainsFunc(patterns, func(p string) bool { return globMatch(p, name) }) || !k.ucaType(a, it) {
			return false
		}
	}
	return true
}

// ucaType reports whether a custom action is for files of its type. A file
// can be of more than one, such as an SVG, which is a picture and text.
func (k *desk) ucaType(a ucaAction, it item) bool {
	if it.dir {
		return a.Dirs != nil
	}
	media, _, _ := strings.Cut(it.mime, "/")
	kinds := []struct {
		of  bool
		set *struct{}
	}{
		{media == "audio", a.Audio},
		{media == "image", a.Image},
		{media == "video", a.Video},
		{k.is(it, "text/plain"), a.Text},
	}
	other := true
	for _, kind := range kinds {
		if kind.of {
			if kind.set != nil {
				return true
			}
			other = false
		}
	}
	return other && a.Other != nil
}

// inRange reports whether n is within a range such as 1, 2-4 or 2-*. An
// empty range takes any count.
func inRange(r string, n int) bool {
	if r == "" {
		return true
	}
	lo, hi, ok := strings.Cut(r, "-")
	if !ok {
		hi = lo
	}
	bound := func(s string, open int) (int, bool) {
		if s = strings.TrimSpace(s); s == "*" {
			return open, true
		}
		v, err := strconv.Atoi(s)
		return v, err == nil
	}
	least, ok1 := bound(lo, 0)
	most, ok2 := bound(hi, n)
	return ok1 && ok2 && least <= n && n <= most
}

// globMatch matches name against a pattern of * and ? the way GLib's
// g_pattern_match_simple does.
func globMatch(pattern, name string) bool {
	p, s := []rune(pattern), []rune(name)
	pi, si, star, mark := 0, 0, -1, 0
	for si < len(s) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == s[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// thunarCommand fills in the codes of a custom action's command and splits
// it the way Thunar does.
func thunarCommand(command string, sel selection) ([]string, error) {
	quoted := func(values []string) string {
		out := make([]string, len(values))
		for i, v := range values {
			out[i] = shellQuote(v)
		}
		return strings.Join(out, " ")
	}
	var dirs, names []string
	for _, it := range sel.items {
		dirs = append(dirs, filepath.Dir(it.path))
		names = append(names, filepath.Base(it.path))
	}
	paths, uris := sel.paths(), sel.uris()
	return splitArgv(fill(command, map[byte]string{
		'f': shellQuote(paths[0]),
		'F': quoted(paths),
		'u': shellQuote(uris[0]),
		'U': quoted(uris),
		'd': shellQuote(dirs[0]),
		'D': quoted(dirs),
		'n': shellQuote(names[0]),
		'N': quoted(names),
	}))
}
