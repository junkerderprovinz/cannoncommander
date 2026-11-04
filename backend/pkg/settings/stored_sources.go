package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// StoredSource is a source as the settings page edits it. Saved sources
// live in sources.json next to the config file and replace the sources of
// the config on the next start, because the index and the path resolver
// are built once.
type StoredSource struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	ReadOnly   bool     `json:"readOnly"`
	Indexed    bool     `json:"indexed"`
	SkipHidden bool     `json:"skipHidden"`
	Exclude    []string `json:"exclude"`
}

// SourcesFileName is the name of the saved sources next to the config file.
const SourcesFileName = "sources.json"

var sourcesFile string

// SourcesFile returns where the saved sources of the running server live.
func SourcesFile() string { return sourcesFile }

// ReadStoredSources returns the saved sources. ok is false when nothing was
// saved yet.
func ReadStoredSources(file string) (list []StoredSource, ok bool, err error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, false, fmt.Errorf("%s: %w", file, err)
	}
	return list, true, nil
}

// WriteStoredSources saves the sources atomically.
func WriteStoredSources(file string, list []StoredSource) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// ExcludeRule turns one exclude pattern into an index rule. A star stands
// for any characters, but the index matches names by prefix, suffix or as a
// whole and paths as a whole, so a star works at the start or the end of a
// name only.
func ExcludeRule(pattern string) (ConditionalRule, error) {
	p := strings.Trim(strings.TrimSpace(pattern), "/")
	switch {
	case p == "" || p == "*":
		return ConditionalRule{}, fmt.Errorf("%q matches everything, switch indexing off instead", pattern)
	case strings.Contains(p, "/"):
		if strings.Contains(p, "*") {
			return ConditionalRule{}, fmt.Errorf("%q: a star works in a name, not in a path", pattern)
		}
		return ConditionalRule{FolderPath: p, FilePath: p}, nil
	case strings.Count(p, "*") > 1 || strings.Contains(strings.Trim(p, "*"), "*"):
		return ConditionalRule{}, fmt.Errorf("%q: a star works at the start or the end of a name only", pattern)
	case strings.HasPrefix(p, "*"):
		s := strings.TrimPrefix(p, "*")
		return ConditionalRule{FileEndsWith: s, FolderEndsWith: s}, nil
	case strings.HasSuffix(p, "*"):
		s := strings.TrimSuffix(p, "*")
		return ConditionalRule{FileStartsWith: s, FolderStartsWith: s}, nil
	}
	return ConditionalRule{FileName: p, FolderName: p}, nil
}

// excludePattern is ExcludeRule read backwards. ok is false for a rule the
// settings page did not write, which is kept as it is.
func excludePattern(r ConditionalRule) (string, bool) {
	switch {
	case r.FileEndsWith != "" && r == ConditionalRule{FileEndsWith: r.FileEndsWith, FolderEndsWith: r.FileEndsWith}:
		return "*" + r.FileEndsWith, true
	case r.FileStartsWith != "" && r == ConditionalRule{FileStartsWith: r.FileStartsWith, FolderStartsWith: r.FileStartsWith}:
		return r.FileStartsWith + "*", true
	case r.FileName != "" && r == ConditionalRule{FileName: r.FileName, FolderName: r.FileName}:
		return r.FileName, true
	case r.FilePath != "" && r == ConditionalRule{FilePath: r.FilePath, FolderPath: r.FolderPath} &&
		strings.Trim(r.FolderPath, "/") == strings.Trim(r.FilePath, "/"):
		return strings.Trim(r.FilePath, "/"), true
	}
	return "", false
}

func isRootRule(r ConditionalRule) bool {
	return r.FolderPath == "/" && r == ConditionalRule{
		FolderPath:            "/",
		Viewable:              r.Viewable,
		IgnoreHidden:          r.IgnoreHidden,
		IgnoreSymlinks:        r.IgnoreSymlinks,
		IgnoreZeroSizeFolders: r.IgnoreZeroSizeFolders,
	}
}

// DescribeSource returns a source as the settings page shows it.
func DescribeSource(s *Source) StoredSource {
	out := StoredSource{Name: s.Name, Path: s.Path, ReadOnly: s.Config.ReadOnly, Indexed: true, Exclude: []string{}}
	for _, r := range s.Config.Rules {
		if isRootRule(r) {
			out.Indexed = out.Indexed && !r.Viewable
			out.SkipHidden = out.SkipHidden || r.IgnoreHidden
			continue
		}
		if p, ok := excludePattern(r); ok {
			out.Exclude = append(out.Exclude, p)
		}
	}
	return out
}

// SourceFromStored builds the source for s. base is the configured source
// with the same path, or nil. Its other settings and the rules the
// settings page cannot show stay as they are.
func SourceFromStored(s StoredSource, base *Source) (*Source, error) {
	out := &Source{Path: s.Path, Name: s.Name}
	if base != nil {
		cp := *base
		out = &cp
		out.Name = s.Name
	}
	out.Config.ReadOnly = s.ReadOnly
	root := ConditionalRule{FolderPath: "/", Viewable: !s.Indexed, IgnoreHidden: s.SkipHidden}
	var rules []ConditionalRule
	if base != nil {
		for _, r := range base.Config.Rules {
			if isRootRule(r) {
				root.IgnoreSymlinks = r.IgnoreSymlinks
				root.IgnoreZeroSizeFolders = r.IgnoreZeroSizeFolders
				continue
			}
			if _, ours := excludePattern(r); !ours {
				rules = append(rules, r)
			}
		}
	}
	if root != (ConditionalRule{FolderPath: "/"}) {
		rules = append(rules, root)
	}
	for _, p := range s.Exclude {
		r, err := ExcludeRule(p)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	out.Config.Rules = rules
	return out, nil
}

// useStoredSources swaps the configured sources for the saved ones before
// they are set up. A broken file is reported and skipped, so the server
// still starts with the sources from the config.
func useStoredSources(configFile string) {
	abs, err := AbsPath(configFile)
	if err != nil {
		return
	}
	sourcesFile = filepath.Join(filepath.Dir(abs), SourcesFileName)
	list, ok, err := ReadStoredSources(sourcesFile)
	if err != nil {
		logger.Errorf("ignoring saved sources: %v", err)
		return
	}
	if !ok || len(list) == 0 {
		return
	}
	byPath := map[string]*Source{}
	for _, s := range Config.Server.Sources {
		if p, err := AbsPath(s.Path); err == nil {
			byPath[p] = s
		}
	}
	next := make([]*Source, 0, len(list))
	for _, s := range list {
		src, err := SourceFromStored(s, byPath[filepath.Clean(s.Path)])
		if err != nil {
			logger.Errorf("ignoring saved sources: %s: %v", s.Name, err)
			return
		}
		next = append(next, src)
	}
	logger.Infof("Sources are the ones saved in %s", sourcesFile)
	Config.Server.Sources = next
}

// SameSources reports whether two lists describe the same sources in the
// same order.
func SameSources(a, b []StoredSource) bool {
	return slices.EqualFunc(a, b, func(x, y StoredSource) bool {
		return x.Name == y.Name && filepath.Clean(x.Path) == filepath.Clean(y.Path) && x.ReadOnly == y.ReadOnly &&
			x.Indexed == y.Indexed && x.SkipHidden == y.SkipHidden && slices.Equal(x.Exclude, y.Exclude)
	})
}
