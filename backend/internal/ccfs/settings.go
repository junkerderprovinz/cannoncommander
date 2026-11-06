package ccfs

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

var diskName = regexp.MustCompile(`^disk[0-9]+$`)

// AdminOnly reports whether p lies on a single disk or a pool. Those bypass
// the user shares and their split rules, so only admins get to see them,
// also when a source reaches them from /mnt or starts deeper inside.
func (r *Resolver) AdminOnly(p string) bool { return r.current().adminOnly(p) }

func (s *sourceSet) adminOnly(p string) bool {
	rest, ok := strings.CutPrefix(p, "/mnt/")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(rest, "/")
	return diskName.MatchString(name) || name == "cache" || slices.Contains(s.named, name)
}

// FromSettings builds a resolver from the configured sources. The source
// path doubles as the mount, because the container sees the host paths.
func FromSettings(cfg *settings.Settings, rules Rules) (*Resolver, error) {
	var sources []Source
	for _, s := range cfg.Server.Sources {
		if s == nil || s.Config.Disabled {
			continue
		}
		mount := MountFor(s.Path)
		sources = append(sources, Source{Name: s.Name, Mount: mount, Root: s.Path, ReadOnly: s.Config.ReadOnly})
	}
	return New(rules, sources...)
}

// MountFor returns the API path of a directory on disk. On Linux that is the
// path itself. A Windows path such as D:\data, which only shows up during
// development, becomes /D/data.
func MountFor(dir string) string {
	mount := filepath.ToSlash(dir)
	if vol := filepath.VolumeName(dir); vol != "" {
		mount = "/" + strings.TrimSuffix(vol, ":") + strings.TrimPrefix(mount, filepath.ToSlash(vol))
	}
	return mount
}
