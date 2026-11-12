package modules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A platform whose modules zip desktop.yml does not build, or release.yml
// does not publish, is one whose copies never find their modules.
func TestTheWorkflowsBuildAndPublishTheModulesOfEveryPlatform(t *testing.T) {
	data, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	src, err := ParseSources(data)
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	desktop, release := read("desktop.yml"), read("release.yml")

	built := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s+modules: (\S+)\n\s+name: (\S+)$`).FindAllStringSubmatch(desktop, -1) {
		platform, name := m[1], m[2]
		goos, goarch, _ := strings.Cut(platform, "/")
		if want := "cannoncommander-modules-" + name + ".zip"; AssetName(goos, goarch) != want {
			t.Errorf("desktop.yml builds %s as %s, but the app asks for %s", platform, want, AssetName(goos, goarch))
		}
		built[platform] = true
	}
	loop := regexp.MustCompile(`for slug in ([^;]+); do\n\s+cp "bundles/cannoncommander-modules-\$\{slug\}\.zip" dist/`).FindStringSubmatch(release)
	if loop == nil {
		t.Fatal("release.yml copies no modules zips into dist")
	}
	published := map[string]bool{}
	for _, slug := range strings.Fields(loop[1]) {
		published[slug] = true
	}
	for platform := range src {
		if strings.HasPrefix(platform, "darwin/") {
			continue
		}
		if !built[platform] {
			t.Errorf("desktop.yml does not build the modules for %s", platform)
		}
		if slug := strings.ReplaceAll(platform, "/", "-"); !published[slug] {
			t.Errorf("release.yml does not publish the modules for %s", platform)
		}
	}
	if len(built) != len(published) {
		t.Errorf("desktop.yml builds %d modules zips, release.yml publishes %d", len(built), len(published))
	}
}
