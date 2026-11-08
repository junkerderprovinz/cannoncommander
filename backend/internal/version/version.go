// Package version holds what the build stamps into the binary.
package version

import (
	"cmp"
	"regexp"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/semver"
)

// Version and CommitSHA are set with -ldflags -X. The image passes its build
// args through as they are, so a build without them stamps empty strings,
// which init replaces.
var (
	Version   string
	CommitSHA string
)

func init() {
	Version, CommitSHA = settle(Version, CommitSHA, vcsRevision())
}

// settle fills in what the build left empty: the commit Go recorded from the
// checkout if there was one, and a version of dev plus that commit.
func settle(version, commit, vcs string) (string, string) {
	commit = cmp.Or(commit, vcs, "unknown")
	if version == "" {
		version = "dev"
		if commit != "unknown" {
			version += "-" + commit[:min(7, len(commit))]
		}
	}
	return version, commit
}

func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}

// describeTail is what git describe appends to a tag for a commit past it or
// a checkout with changes.
var describeTail = regexp.MustCompile(`-(\d+-g[0-9a-f]+|dirty)$`)

// Release returns Version as a tag such as v1.4.0 if the binary was built from
// a release tag. For any other build it reports false, so it is never offered
// an update to the release it was built after.
func Release() (string, bool) {
	v := "v" + strings.TrimPrefix(Version, "v")
	if !semver.IsValid(v) || describeTail.MatchString(v) {
		return "", false
	}
	return v, true
}
