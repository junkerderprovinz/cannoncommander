package mcpapi

import (
	"regexp"
	"sort"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

var (
	windowsPath = regexp.MustCompile(`(?i)\b[a-z]:\\[^\s"']*[^\s"'.,:;)]`)
	uncPath     = regexp.MustCompile(`\\\\[^\s"'\\]+\\[^\s"']*[^\s"'.,:;)]`)
	credentials = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
	// unixPath is a path that starts a word. A slash after a colon is the
	// start of an address, not a path, and stays.
	unixPath = regexp.MustCompile(`(^|[\s"'(=,])(/[^\s"',)]*)`)
)

// scrub takes the host out of a text an assistant will read: a path on disk
// becomes the path of the API that shows the same file, a path the account
// cannot see becomes [path], and an address loses its credentials. Error
// texts from the disk name what the server opened, such as a pool below
// /mnt/cache, which is no business of an account without that source.
func (s *Server) scrub(u *users.User, text string) string {
	text = windowsPath.ReplaceAllString(text, "[path]")
	text = uncPath.ReplaceAllString(text, "[path]")
	text = credentials.ReplaceAllString(text, "://[redacted]@")
	sources := s.deps.FS.Sources()
	// The deepest root first, so a source inside another maps to its own mount.
	sort.Slice(sources, func(i, j int) bool { return len(sources[i].Root) > len(sources[j].Root) })
	return unixPath.ReplaceAllStringFunc(text, func(m string) string {
		at := strings.Index(m, "/")
		lead, p := m[:at], m[at:]
		tail := ""
		for len(p) > 1 && strings.ContainsRune(".:;", rune(p[len(p)-1])) {
			p, tail = p[:len(p)-1], p[len(p)-1:]+tail
		}
		if shown, ok := shownAs(sources, u, p); ok {
			return lead + shown + tail
		}
		return lead + "[path]" + tail
	})
}

// shownAs returns p as the account knows it: a path on disk below a source's
// root turns into the same path below its mount, and a path of the API stays.
// ok is false for anything outside the sources the account has.
func shownAs(sources []ccfs.Source, u *users.User, p string) (string, bool) {
	for _, src := range sources {
		if src.Root != src.Mount && src.Root != "" && below(p, src.Root) {
			p = strings.TrimSuffix(src.Mount, "/") + strings.TrimPrefix(p, strings.TrimSuffix(src.Root, "/"))
			break
		}
	}
	for _, src := range sources {
		if below(p, src.Mount) && sees(u, src) {
			return p, true
		}
	}
	return "", false
}

func below(p, base string) bool {
	base = strings.TrimSuffix(base, "/")
	return base == "" || p == base || strings.HasPrefix(p, base+"/")
}

// sees reports whether u has src at all: an admin has every source, anybody
// else the ones a scope gives them that are not kept for admins.
func sees(u *users.User, src ccfs.Source) bool {
	if u.Permissions.Admin {
		return true
	}
	if src.AdminOnly {
		return false
	}
	for _, bs := range u.BackendScopes {
		if bs.Path == src.Root {
			return true
		}
	}
	return false
}
