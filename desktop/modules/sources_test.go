package modules

import (
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

func TestSourcesJSONPinsTheThreeModulesForEveryPlatformTheAppUpdatesOn(t *testing.T) {
	data, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := ParseSources(data)
	if err != nil {
		t.Fatal(err)
	}
	for platform := range update.Assets {
		mods, ok := src[platform]
		if !ok {
			t.Errorf("sources.json has nothing for %s", platform)
			continue
		}
		for _, name := range []string{"ffmpeg", "7zip", "mupdf"} {
			if _, ok := mods[name]; !ok {
				t.Errorf("%s has no %s", platform, name)
			}
		}
		if mods["ffmpeg"].Env["CC_FFMPEG_PATH"] == "" {
			t.Errorf("%s: ffmpeg does not set CC_FFMPEG_PATH", platform)
		}
		if mods["7zip"].Env["CC_7ZZ"] == "" {
			t.Errorf("%s: 7-Zip does not set CC_7ZZ", platform)
		}
	}
	for platform := range src {
		if _, ok := update.Assets[platform]; !ok {
			t.Errorf("sources.json names %s, which the app is not built for", platform)
		}
	}
}

func TestEveryModuleKeepsALicence(t *testing.T) {
	data, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := ParseSources(data)
	if err != nil {
		t.Fatal(err)
	}
	for platform, mods := range src {
		for name, m := range mods {
			found := false
			for _, s := range m.Sources {
				for pattern := range s.Keep {
					upper := strings.ToUpper(pattern)
					found = found || strings.Contains(upper, "LICENSE") || strings.Contains(upper, "COPYING")
				}
			}
			if !found {
				t.Errorf("%s %s keeps no licence file", platform, name)
			}
		}
	}
}

func TestParseSourcesRefusesWhatCannotBeFetchedOrPlacedSafely(t *testing.T) {
	const good = `"version": "1", "path": "bin", "sources": [{"url": "https://example.org/a.zip", "sha256": "` +
		`0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "keep": {"*/bin/*": "bin"}}]`
	if _, err := ParseSources([]byte(`{"linux/amd64": {"tool": {` + good + `}}}`)); err != nil {
		t.Fatalf("a sound entry was refused: %v", err)
	}
	for name, entry := range map[string]string{
		"no platform":      `{"linux": {"tool": {` + good + `}}}`,
		"plain http":       `{"linux/amd64": {"tool": {` + strings.Replace(good, "https:", "http:", 1) + `}}}`,
		"short checksum":   `{"linux/amd64": {"tool": {` + strings.Replace(good, "0123456789abcdef\"", "\"", 1) + `}}}`,
		"upper-case hash":  `{"linux/amd64": {"tool": {` + strings.Replace(good, "abcdef\"", "ABCDEF\"", 1) + `}}}`,
		"keep leaves":      `{"linux/amd64": {"tool": {` + strings.Replace(good, `"bin"}`, `"../bin"}`, 1) + `}}}`,
		"absolute path":    `{"linux/amd64": {"tool": {` + strings.Replace(good, `"path": "bin"`, `"path": "/usr/bin"`, 1) + `}}}`,
		"bad pattern":      `{"linux/amd64": {"tool": {` + strings.Replace(good, `"*/bin/*"`, `"[bin"`, 1) + `}}}`,
		"no version":       `{"linux/amd64": {"tool": {` + strings.Replace(good, `"version": "1"`, `"version": ""`, 1) + `}}}`,
		"foreign variable": `{"linux/amd64": {"tool": {"env": {"PATH": "bin"}, ` + good + `}}}`,
	} {
		if _, err := ParseSources([]byte(entry)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
