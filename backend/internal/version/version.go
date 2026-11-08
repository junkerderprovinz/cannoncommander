// Package version holds what the build stamps into the binary.
package version

import (
	"cmp"
	_ "embed"
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

// archive is filled in by git archive through export-subst, so source handed
// over as an archive, without its .git, still knows where it came from.
//
//go:embed archive.txt
var archive string

func init() {
	Version, CommitSHA = settle(Version, CommitSHA, vcsRevision(), archive)
}

// settle fills in what the build left empty: the describe and commit git
// archive wrote, the commit Go recorded from the checkout, and failing those
// a version of dev plus the commit.
func settle(version, commit, vcs, archive string) (string, string) {
	var described, archived string
	if !strings.HasPrefix(archive, "$Format") {
		described, archived, _ = strings.Cut(strings.TrimRight(archive, "\r\n"), " ")
	}
	commit = cmp.Or(commit, archived, vcs, "unknown")
	version = cmp.Or(version, described)
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

// Link returns the release page of a release build, and the commit of any
// other build whose commit is known.
func Link() string {
	const repo = "https://github.com/junkerderprovinz/cannoncommander"
	if v, ok := Release(); ok {
		return repo + "/releases/tag/" + v
	}
	if CommitSHA == "unknown" {
		return repo
	}
	return repo + "/commit/" + CommitSHA
}
