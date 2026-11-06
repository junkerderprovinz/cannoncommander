package ccfs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// unraidMnt lays out a small /mnt with the user shares, two pools and two
// disks, and returns its folder.
func unraidMnt(t *testing.T, dirs ...string) string {
	t.Helper()
	mnt := t.TempDir()
	for _, d := range append([]string{"user/filme", "cache/appdata", "fastcache", "disk1/filme", "disk2", "remotes"}, dirs...) {
		if err := os.MkdirAll(filepath.Join(mnt, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return mnt
}

func sharesAndRemotes(t *testing.T, mnt string, rules Rules) *Resolver {
	t.Helper()
	r, err := New(rules,
		Source{Name: "Shares", Mount: "/mnt/user", Root: filepath.Join(mnt, "user")},
		Source{Name: "Fernziele", Mount: "/mnt/remotes", Root: filepath.Join(mnt, "remotes")},
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDisksAndPoolsAppearForAdminsAfterTheUserShares(t *testing.T) {
	mnt := unraidMnt(t)
	r := sharesAndRemotes(t, mnt, nil).WithDevices(mnt, "cache", "fastcache", "disk1", "disk2")
	shares := users.BackendScope{Path: filepath.Join(mnt, "user"), Scope: "/", Permissions: allPerms}
	jdp, anna := user("jdp", true, shares), user("anna", false, shares)

	if got := r.VirtualChildren("/mnt"); !slices.Equal(got, []string{"user", "cache", "fastcache", "disk1", "disk2", "remotes"}) {
		t.Errorf("/mnt lists %v", got)
	}
	labels := map[string]string{}
	for _, s := range r.Sources() {
		if s.Device {
			labels[s.Mount] = s.Name
			if !s.AdminOnly {
				t.Errorf("%s is open to everyone", s.Mount)
			}
		}
	}
	if labels["/mnt/disk1"] != "Disk 1" || labels["/mnt/cache"] != "Cache" || labels["/mnt/fastcache"] != "Fastcache" {
		t.Errorf("labels %v", labels)
	}

	if _, err := r.Check(jdp, "/mnt/disk1/filme", View|Create|Delete); err != nil {
		t.Errorf("admin on a disk: %v", err)
	}
	if _, err := r.Check(anna, "/mnt/disk1/filme", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("plain user on a disk: %v", err)
	}
	dir, err := r.Check(anna, "/mnt", View)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cache", "fastcache", "disk1", "disk2"} {
		if r.Visible(anna, dir, name) {
			t.Errorf("anna sees %s in /mnt", name)
		}
	}
}

func TestAConfiguredSourceOnAPoolWinsOverTheDevice(t *testing.T) {
	mnt := unraidMnt(t)
	base, err := New(nil,
		Source{Name: "Shares", Mount: "/mnt/user", Root: filepath.Join(mnt, "user")},
		Source{Name: "Disk 1", Mount: "/mnt/cache", Root: filepath.Join(mnt, "cache")},
	)
	if err != nil {
		t.Fatal(err)
	}
	r := base.WithDevices(mnt, "cache", "disk1")
	var names []string
	for _, s := range r.Sources() {
		names = append(names, s.Name)
		if s.Mount == "/mnt/cache" && (s.Device || !s.AdminOnly) {
			t.Errorf("the cache source: %+v", s)
		}
	}
	if !slices.Contains(names, "disk1") || slices.Contains(names, "Cache") {
		t.Errorf("sources %v", names)
	}
}

func TestTheDataFolderIsClosedOnEveryDiskThatHoldsAppdata(t *testing.T) {
	mnt := unraidMnt(t, "user/appdata/cannoncommander", "cache/appdata/cannoncommander")
	r := sharesAndRemotes(t, mnt, nil).WithDevices(mnt, "cache", "disk1")
	if err := os.WriteFile(filepath.Join(mnt, "user", "appdata", "cannoncommander", dataMarker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r = r.WithDataFolder(t.TempDir())
	jdp := user("jdp", true)
	for _, p := range []string{"/mnt/user/appdata/cannoncommander", "/mnt/cache/appdata/cannoncommander/database.db", "/mnt/disk1/appdata/cannoncommander"} {
		if _, err := r.Check(jdp, p, View); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v", p, err)
		}
	}
}

// exclusive is the share config of a server where code lives on fastcache only.
type exclusive struct {
	user  string
	pools map[string]string
}

func (x exclusive) UserShares() string { return x.user }

func (x exclusive) ExclusivePool(share string) (string, bool) {
	p, ok := x.pools[share]
	return p, ok
}

// exclusiveMnt makes /mnt/user/code a link to ../fastcache/code the way Unraid
// does for an exclusive share, next to links that lead to a pool on their own.
func exclusiveMnt(t *testing.T, rules Rules) (*Resolver, string) {
	t.Helper()
	mnt := unraidMnt(t, "fastcache/code/src", "fastcache/other", "user/media")
	if err := os.WriteFile(filepath.Join(mnt, "fastcache", "code", "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"user/code":           "../fastcache/code",
		"user/other":          "../fastcache/other",
		"user/media/pool":     "../../fastcache/code",
		"fastcache/code/back": "../../user/filme",
		"fastcache/code/up":   "..",
	}
	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(mnt, filepath.FromSlash(link))); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	x := exclusive{user: filepath.Join(mnt, "user"), pools: map[string]string{"code": "fastcache"}}
	return sharesAndRemotes(t, mnt, rules).WithDevices(mnt, "cache", "fastcache").WithExclusive(x), mnt
}

func TestAnExclusiveShareActsLikeAFolderOfTheUserShares(t *testing.T) {
	r, mnt := exclusiveMnt(t, nil)
	anna := user("anna", false, users.BackendScope{Path: filepath.Join(mnt, "user"), Scope: "/", Permissions: allPerms})

	for _, p := range []string{"/mnt/user/code", "/mnt/user/code/readme.txt", "/mnt/user/code/src", "/mnt/user/code/neu/datei.txt"} {
		loc, err := r.Reach(anna, p, View|Create)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		landed, err := r.Followed(loc)
		if err != nil || landed.Path != p {
			t.Errorf("%s lands at %q, %v", p, landed.Path, err)
		}
		if err := Contained(loc); err != nil {
			t.Errorf("Contained(%s) = %v", p, err)
		}
	}
	loc, _ := r.Resolve("/mnt/user/code/readme.txt")
	if data, err := os.ReadFile(loc.Real); err != nil || string(data) != "hi" {
		t.Errorf("reading %s: %q, %v", loc.Real, data, err)
	}
	root, _ := r.Resolve("/mnt/user")
	if info, err := os.Lstat(r.RealIn(root, "code")); err != nil || !info.IsDir() {
		t.Errorf("the share shows up as a link: %v", err)
	}
	if got := r.RealIn(root, "filme"); got != filepath.Join(mnt, "user", "filme") {
		t.Errorf("RealIn(filme) = %s", got)
	}
}

func TestLinksOutOfTheUserSharesStayRefused(t *testing.T) {
	r, mnt := exclusiveMnt(t, nil)
	jdp := user("jdp", true, users.BackendScope{Path: filepath.Join(mnt, "user"), Scope: "/", Permissions: allPerms})

	for p, want := range map[string]error{
		"/mnt/user/other":           ErrEscape,
		"/mnt/user/media/pool":      ErrEscape,
		"/mnt/user/media/pool/src":  ErrEscape,
		"/mnt/user/code/up":         ErrEscape,
		"/mnt/user/code/up/other":   ErrEscape,
		"/mnt/user/code/back":       nil,
		"/mnt/fastcache/code/src":   nil,
		"/mnt/fastcache/code/back":  ErrEscape,
		"/mnt/user/code/../../boot": ErrBadPath,
	} {
		_, err := r.Reach(jdp, p, View)
		if !errors.Is(err, want) && !(want == nil && err == nil) {
			t.Errorf("Reach(%s) = %v, want %v", p, err, want)
		}
	}
	loc, _ := r.Resolve("/mnt/user/code/back")
	if landed, err := r.Followed(loc); err != nil || landed.Path != "/mnt/user/filme" {
		t.Errorf("a link from the share to another share lands at %q, %v", landed.Path, err)
	}
}

func TestRulesOnAnExclusiveShareGoByItsUserSharePath(t *testing.T) {
	r, mnt := exclusiveMnt(t, pathRules{hidden: "/mnt/user/code/src", readOnly: "/mnt/user/code"})
	anna := user("anna", false, users.BackendScope{Path: filepath.Join(mnt, "user"), Scope: "/", Permissions: allPerms})

	if _, err := r.Reach(anna, "/mnt/user/code/src", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("hidden folder in the share: %v", err)
	}
	if _, err := r.Reach(anna, "/mnt/user/code/readme.txt", Delete); !errors.Is(err, ErrReadOnly) {
		t.Errorf("read only share: %v", err)
	}
	if _, err := r.Reach(anna, "/mnt/fastcache/code/readme.txt", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("the pool behind the share: %v", err)
	}
}
