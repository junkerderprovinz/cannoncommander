package fsapi

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/shortcut"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// shortcutKinds are the shortcut files the desktop of each system opens:
// Explorer follows .lnk and .url, Nautilus and Dolphin start .desktop
// launchers, and the desktop app opens the address of a .url on Linux as
// well. The container leaves all of them as plain files.
var shortcutKinds = map[string]bool{
	shortcut.Lnk: runtime.GOOS == "windows", shortcut.URL: runtime.GOOS == "windows" || runtime.GOOS == "linux",
	shortcut.Desktop: runtime.GOOS == "linux",
}

// readShortcut leaves a file in the cloud alone, which reading would fetch.
func readShortcut(real, name string, attrs uint32) *shortcut.Shortcut {
	if !shortcutKinds[shortcut.KindOf(name)] || !desktop.Active() || cloudState(attrs, false) == "online" {
		return nil
	}
	sc, err := shortcut.Read(real)
	if err != nil {
		return nil
	}
	return &sc
}

// apiPathOf turns a path on disk into the path of the API, when one of the
// sources holds it.
func (h handlers) apiPathOf(real string) string {
	if !filepath.IsAbs(real) {
		return ""
	}
	for _, s := range h.fs.Sources() {
		rel, err := filepath.Rel(s.Root, real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return path.Join(s.Mount, filepath.ToSlash(rel))
	}
	return ""
}

// targets puts where a link or a shortcut leads into API paths, so the
// panel can go there. A link on Linux already names the path the API uses.
func (h handlers) targets(item *Item) {
	if item.Link != "" && filepath.VolumeName(filepath.FromSlash(item.Link)) != "" {
		if p := h.apiPathOf(filepath.FromSlash(item.Link)); p != "" {
			item.Link = p
		}
	}
	if sc := item.Shortcut; sc != nil && sc.Target != "" {
		sc.Path = h.apiPathOf(sc.Target)
	}
}
