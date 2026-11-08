package main

import (
	"net/url"
	"strings"
)

// fileURI is the file:// URI of an absolute path.
func fileURI(p string) string {
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// filePath is the path of a file:// URI on this computer, or false for any
// other URI.
func filePath(uri string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil || u.Scheme != "file" || u.Host != "" && u.Host != "localhost" || u.Path == "" {
		return "", false
	}
	return u.Path, true
}

// clipFormats writes files the ways Linux file managers read them: the list
// GNOME and its kin share, which starts with copy or cut, the plain URI
// list, KDE's mark for a cut and the paths as text.
func clipFormats(paths []string, cut bool) (gnome, uris, kde, text string) {
	list := make([]string, len(paths))
	for i, p := range paths {
		list[i] = fileURI(p)
	}
	verb, kde := "copy", "0"
	if cut {
		verb, kde = "cut", "1"
	}
	return verb + "\n" + strings.Join(list, "\n"), strings.Join(list, "\r\n") + "\r\n", kde, strings.Join(paths, "\n")
}

// parseGnomeFiles reads x-special/gnome-copied-files: copy or cut on the
// first line and a URI on each line after it.
func parseGnomeFiles(raw string) (paths []string, cut bool) {
	verb, rest, _ := strings.Cut(strings.TrimRight(raw, "\x00"), "\n")
	return parseURIList(rest), strings.TrimSpace(verb) == "cut"
}

// parseURIList reads text/uri-list, one URI per line and lines with # as
// comments, keeping the files on this computer.
func parseURIList(raw string) []string {
	var paths []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.Trim(line, " \t\x00")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if p, ok := filePath(line); ok {
			paths = append(paths, p)
		}
	}
	return paths
}
