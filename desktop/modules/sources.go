// Package modules brings the helper programs the desktop app runs: ffmpeg and
// ffprobe for thumbnails, video and media details, 7-Zip for 7z archives and
// MuPDF's mutool for PDF pages. CI puts them together per platform from the
// upstream builds pinned in sources.json, the release carries them as one zip
// per platform, and the app installs the zip of its own version into a
// modules folder next to the program.
package modules

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Sources maps a platform such as "windows/amd64" to its modules by name.
type Sources map[string]map[string]Module

// Module is one helper program for one platform. In modules.json it comes
// without its sources.
type Module struct {
	Version string `json:"version"`
	// Path is the folder inside the module that goes on the PATH.
	Path string `json:"path"`
	// Env names the variables the server reads, each set to a path inside
	// the module.
	Env     map[string]string `json:"env,omitempty"`
	Sources []Source          `json:"sources,omitempty"`
}

// Source is one upstream file a module takes programs and licences from.
type Source struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	// Build is run in the unpacked archive before Keep picks from it, for a
	// program that comes as source only.
	Build []string `json:"build,omitempty"`
	// Keep maps patterns of paths in the archive, in path.Match's syntax, to
	// the folder in the module the matching files go to. A file that is no
	// archive is matched by its name.
	Keep map[string]string `json:"keep"`
}

// ParseSources reads a sources.json and checks that every entry can be
// fetched, verified and placed.
func ParseSources(data []byte) (Sources, error) {
	var s Sources
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	for platform, mods := range s {
		goos, goarch, ok := strings.Cut(platform, "/")
		if !ok || goos == "" || goarch == "" {
			return nil, fmt.Errorf("%q is not a platform like windows/amd64", platform)
		}
		for name, m := range mods {
			if err := m.check(); err != nil {
				return nil, fmt.Errorf("%s %s: %w", platform, name, err)
			}
		}
	}
	return s, nil
}

func (m Module) check() error {
	if m.Version == "" {
		return fmt.Errorf("no version")
	}
	if !inside(m.Path) {
		return fmt.Errorf("path %q leaves the module", m.Path)
	}
	for k, v := range m.Env {
		if !strings.HasPrefix(k, "CC_") || !inside(v) {
			return fmt.Errorf("env %s=%s is not a CC_ variable naming a path inside the module", k, v)
		}
	}
	if len(m.Sources) == 0 {
		return fmt.Errorf("no sources")
	}
	for _, src := range m.Sources {
		if err := src.check(); err != nil {
			return fmt.Errorf("%s: %w", src.URL, err)
		}
	}
	return nil
}

func (s Source) check() error {
	u, err := url.Parse(s.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("not an https address")
	}
	if b, err := hex.DecodeString(s.SHA256); err != nil || len(b) != 32 || s.SHA256 != strings.ToLower(s.SHA256) {
		return fmt.Errorf("sha256 %q is not 64 lower-case hex digits", s.SHA256)
	}
	if len(s.Keep) == 0 {
		return fmt.Errorf("keeps nothing")
	}
	for pattern, dir := range s.Keep {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("keep %q: %w", pattern, err)
		}
		if !inside(dir) {
			return fmt.Errorf("keep %q puts files in %q, outside the module", pattern, dir)
		}
	}
	return nil
}

// inside reports whether p is a relative slash path that stays within the
// folder it is taken from.
func inside(p string) bool {
	return p != "" && !strings.Contains(p, `\`) && path.Clean(p) == p && !path.IsAbs(p) && p != ".." && !strings.HasPrefix(p, "../")
}

// Record is modules.json at the top of a modules folder: the release the
// modules came with, the platform they run on and each module without its
// sources.
type Record struct {
	Release  string            `json:"release"`
	Platform string            `json:"platform"`
	Modules  map[string]Module `json:"modules"`
}

// RecordFile is the name of the record in a modules folder.
const RecordFile = "modules.json"

// AssetName is the release file holding the modules for goos/goarch. macOS
// has none, since every Mac bundle carries its modules.
func AssetName(goos, goarch string) string {
	return "cannoncommander-modules-" + goos + "-" + goarch + ".zip"
}
