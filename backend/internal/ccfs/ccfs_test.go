package ccfs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

type fakeRoots struct {
	user, cache, disk1, boot string
}

func newResolver(t *testing.T, access AccessGate) (*Resolver, fakeRoots) {
	t.Helper()
	base := t.TempDir()
	roots := fakeRoots{
		user:  filepath.Join(base, "user"),
		cache: filepath.Join(base, "cache"),
		disk1: filepath.Join(base, "disk1"),
		boot:  filepath.Join(base, "boot"),
	}
	for _, d := range []string{roots.user, roots.cache, roots.disk1, roots.boot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r, err := New(access,
		Source{Name: "Shares", Mount: "/mnt/user", Root: roots.user},
		Source{Name: "Cache", Mount: "/mnt/cache", Root: roots.cache, AdminOnly: true},
		Source{Name: "Disk 1", Mount: "/mnt/disk1", Root: roots.disk1, AdminOnly: true},
		Source{Name: "USB-Stick", Mount: "/boot", Root: roots.boot, ReadOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	return r, roots
}

var allPerms = users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}

func user(name string, admin bool, scopes ...users.BackendScope) *users.User {
	u := &users.User{BackendScopes: scopes}
	u.Username = name
	u.Permissions.Admin = admin
	return u
}

func TestCleanAcceptsOnlyAbsolutePathsWithoutDotDot(t *testing.T) {
	cases := []struct {
		in, want string
		err      bool
	}{
		{in: "/", want: "/"},
		{in: "/mnt//user/", want: "/mnt/user"},
		{in: "/mnt/user/./filme", want: "/mnt/user/filme"},
		{in: "/mnt/user/a.zip::", want: "/mnt/user/a.zip::/"},
		{in: "/mnt/user/a.zip::Strand//IMG.jpg", want: "/mnt/user/a.zip::/Strand/IMG.jpg"},
		{in: "", err: true},
		{in: "mnt/user", err: true},
		{in: "/mnt/user/../../etc", err: true},
		{in: "/mnt/user/a.zip::/../x", err: true},
		{in: "/a.zip::/b.zip::/c", err: true},
		{in: "/::", err: true},
		{in: "/mnt/user/\x00", err: true},
	}
	for _, c := range cases {
		got, err := Clean(c.in)
		if c.err {
			if err == nil {
				t.Errorf("Clean(%q) = %q, want an error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("Clean(%q) = %q, %v, want %q", c.in, got, err, c.want)
		}
	}
}

func TestResolveMapsMountsToRoots(t *testing.T) {
	r, roots := newResolver(t, nil)

	loc, err := r.Resolve("/mnt/user/filme/Sintel (2010)")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Source.Name != "Shares" || loc.Index != "/filme/Sintel (2010)" {
		t.Fatalf("loc = %+v", loc)
	}
	if want := filepath.Join(roots.user, "filme", "Sintel (2010)"); loc.Real != want {
		t.Fatalf("Real = %q, want %q", loc.Real, want)
	}

	loc, err = r.Resolve("/mnt/user/downloads/fotos.zip::/Strand/IMG_0101.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if !loc.InArchive() || loc.Inner != "/Strand/IMG_0101.jpg" || loc.Index != "/downloads/fotos.zip" {
		t.Fatalf("archive loc = %+v", loc)
	}
}

func TestResolveKnowsTheVirtualParents(t *testing.T) {
	r, _ := newResolver(t, nil)
	for _, p := range []string{"/", "/mnt"} {
		loc, err := r.Resolve(p)
		if err != nil || !loc.Virtual() {
			t.Errorf("Resolve(%q) = %+v, %v, want a virtual folder", p, loc, err)
		}
	}
	if _, err := r.Resolve("/etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve outside every source: err = %v", err)
	}
	if got := r.VirtualChildren("/"); !slices.Equal(got, []string{"boot", "mnt"}) {
		t.Errorf("children of / = %v", got)
	}
	if got := r.VirtualChildren("/mnt"); !slices.Equal(got, []string{"cache", "disk1", "user"}) {
		t.Errorf("children of /mnt = %v", got)
	}
}

func TestNewRejectsNestedSources(t *testing.T) {
	_, err := New(nil, Source{Name: "a", Mount: "/mnt/user", Root: "/x"}, Source{Name: "b", Mount: "/mnt/user/filme", Root: "/y"})
	if err == nil {
		t.Fatal("nested sources were accepted")
	}
}

func TestCheckFollowsScopesAndRights(t *testing.T) {
	r, roots := newResolver(t, nil)
	anna := user("anna", false, users.BackendScope{Path: roots.user, Scope: "/fotos", Permissions: users.SourceFilePermissions{View: true, Download: true}})
	admin := user("jdp", true,
		users.BackendScope{Path: roots.user, Scope: "/", Permissions: allPerms},
		users.BackendScope{Path: roots.disk1, Scope: "/", Permissions: allPerms},
		users.BackendScope{Path: roots.boot, Scope: "/", Permissions: allPerms},
	)
	cases := []struct {
		u    *users.User
		path string
		perm Perm
		want error
	}{
		{anna, "/", View, nil},
		{anna, "/mnt", View, nil},
		{anna, "/mnt/user", View, nil},
		{anna, "/mnt/user", Create, ErrForbidden},
		{anna, "/mnt/user/fotos/2025", View | Download, nil},
		{anna, "/mnt/user/fotos/2025", Delete, ErrForbidden},
		{anna, "/mnt/user/backups", View, ErrForbidden},
		{anna, "/boot", View, ErrForbidden},
		{anna, "/mnt/disk1", View, ErrForbidden},
		{admin, "/mnt/disk1/filme", Delete, nil},
		{admin, "/boot/config", View, nil},
		{admin, "/boot/config/go", Modify, ErrReadOnly},
		{admin, "/mnt/user/a.zip::/x", View, nil},
		{admin, "/mnt/user/a.zip::/x", Create, ErrReadOnly},
		{admin, "/mnt", Create, ErrForbidden},
		{nil, "/mnt/user", View, ErrForbidden},
	}
	for _, c := range cases {
		name := "nil"
		if c.u != nil {
			name = c.u.Username
		}
		_, err := r.Check(c.u, c.path, c.perm)
		if !errors.Is(err, c.want) && !(c.want == nil && err == nil) {
			t.Errorf("Check(%s, %q, %v) = %v, want %v", name, c.path, c.perm, err, c.want)
		}
	}
}

type denyRule struct {
	source, index, user string
}

func (d denyRule) AccessPermitted(source string, p utils.IndexPath, username string) bool {
	return !(source == d.source && username == d.user && strings.HasPrefix(p.String(), d.index))
}

func TestCheckHonoursAccessRules(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "user")
	r, err := New(denyRule{source: root, index: "/backups/", user: "anna"}, Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	anna := user("anna", false, users.BackendScope{Path: root, Scope: "/", Permissions: allPerms})

	if _, err := r.Check(anna, "/mnt/user/backups/flash.zip", View); !errors.Is(err, ErrForbidden) {
		t.Fatalf("denied path: err = %v", err)
	}
	dir, err := r.Check(anna, "/mnt/user", View)
	if err != nil {
		t.Fatal(err)
	}
	if r.Visible(anna, dir, "backups") {
		t.Error("denied folder is visible")
	}
	if !r.Visible(anna, dir, "filme") {
		t.Error("allowed folder is hidden")
	}
}

// pathRules hides one folder and makes another read only, for anna only.
type pathRules struct{ hidden, readOnly string }

func (pathRules) AccessPermitted(string, utils.IndexPath, string) bool { return true }

func (p pathRules) Effect(u *users.User, path string) Effect {
	switch {
	case u.Username != "anna":
		return Open
	case path == p.hidden || strings.HasPrefix(path, p.hidden+"/"):
		return Hidden
	case path == p.readOnly || strings.HasPrefix(path, p.readOnly+"/"):
		return ReadOnly
	}
	return Open
}

func TestCheckHonoursTheAppRules(t *testing.T) {
	r, roots := newResolver(t, pathRules{hidden: "/mnt/user/backups", readOnly: "/mnt/user/fotos"})
	anna := user("anna", false, users.BackendScope{Path: roots.user, Scope: "/", Permissions: allPerms})
	jdp := user("jdp", false, users.BackendScope{Path: roots.user, Scope: "/", Permissions: allPerms})

	if _, err := r.Check(anna, "/mnt/user/backups/flash.zip", View); !errors.Is(err, ErrForbidden) {
		t.Fatalf("hidden path: err = %v", err)
	}
	if _, err := r.Check(anna, "/mnt/user/backups/flash.zip::/config", View); !errors.Is(err, ErrForbidden) {
		t.Fatalf("archive in a hidden folder: err = %v", err)
	}
	if _, err := r.Check(anna, "/mnt/user/fotos/a.jpg", View|Download); err != nil {
		t.Fatalf("reading a read only folder: %v", err)
	}
	if _, err := r.Check(anna, "/mnt/user/fotos/a.jpg", Delete); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("deleting in a read only folder: err = %v", err)
	}
	if _, err := r.Check(jdp, "/mnt/user/backups", Delete); err != nil {
		t.Fatalf("the rules are only for anna: %v", err)
	}
	dir, err := r.Check(anna, "/mnt/user", View)
	if err != nil {
		t.Fatal(err)
	}
	if r.Visible(anna, dir, "backups") || !r.Visible(anna, dir, "fotos") {
		t.Error("only the hidden folder may drop out of the listing")
	}
}

func TestDisksAndPoolsStayWithAdmins(t *testing.T) {
	base := t.TempDir()
	mnt := filepath.Join(base, "mnt")
	for _, d := range []string{"user/filme", "disk1/filme", "cache/appdata", "fastcache/vm", "remotes/nas"} {
		if err := os.MkdirAll(filepath.Join(mnt, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plain, err := New(nil, Source{Name: "Server", Mount: "/mnt", Root: mnt})
	if err != nil {
		t.Fatal(err)
	}
	r := plain.WithDevices("cache", "fastcache", "disk1")
	all := users.BackendScope{Path: mnt, Scope: "/", Permissions: allPerms}
	anna, jdp := user("anna", false, all), user("jdp", true, all)

	for _, p := range []string{"/mnt/disk1", "/mnt/disk1/filme", "/mnt/cache/appdata", "/mnt/fastcache/vm"} {
		if _, err := r.Check(anna, p, View); !errors.Is(err, ErrForbidden) {
			t.Errorf("anna on %s: err = %v", p, err)
		}
		if _, err := r.Check(jdp, p, View|Modify); err != nil {
			t.Errorf("jdp on %s: %v", p, err)
		}
	}
	if _, err := r.Check(anna, "/mnt/user/filme", View|Modify); err != nil {
		t.Errorf("anna in the shares: %v", err)
	}
	dir, err := r.Check(anna, "/mnt", View)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, name := range []string{"user", "disk1", "cache", "fastcache", "remotes"} {
		if r.Visible(anna, dir, name) {
			seen = append(seen, name)
		}
	}
	if !slices.Equal(seen, []string{"user", "remotes"}) {
		t.Errorf("anna sees %v in /mnt", seen)
	}
	if !r.AdminOnly("/mnt/fastcache") || plain.AdminOnly("/mnt/fastcache") || r.AdminOnly("/mnt/user/cache") {
		t.Error("a pool counts only when Unraid names it")
	}
}

func TestSourcesOnADiskAreForAdmins(t *testing.T) {
	r, err := New(nil, Source{Name: "Filme", Mount: "/mnt/disk3/filme", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Sources()[0].AdminOnly {
		t.Error("a source below /mnt/disk3 is open to everyone")
	}
}

func TestVisibleShowsOnlyTheWayToTheScope(t *testing.T) {
	r, roots := newResolver(t, nil)
	anna := user("anna", false, users.BackendScope{Path: roots.user, Scope: "/fotos", Permissions: allPerms})

	root, _ := r.Check(anna, "/", View)
	if !r.Visible(anna, root, "mnt") || r.Visible(anna, root, "boot") {
		t.Error("wrong entries at /")
	}
	mnt, _ := r.Check(anna, "/mnt", View)
	if !r.Visible(anna, mnt, "user") || r.Visible(anna, mnt, "cache") {
		t.Error("wrong entries at /mnt")
	}
	shares, _ := r.Check(anna, "/mnt/user", View)
	if !r.Visible(anna, shares, "fotos") || r.Visible(anna, shares, "filme") {
		t.Error("wrong entries at /mnt/user")
	}
}

func TestContainedCatchesSymlinksThatLeave(t *testing.T) {
	r, roots := newResolver(t, nil)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(roots.user, "away")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(roots.user, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(roots.user, "filme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(roots.user, "filme"), filepath.Join(roots.user, "kino")); err != nil {
		t.Fatal(err)
	}

	for p, want := range map[string]error{
		"/mnt/user/away/secret.txt":  ErrEscape,
		"/mnt/user/dangling":         ErrEscape,
		"/mnt/user/kino/neu.mkv":     nil,
		"/mnt/user/filme/noch/nicht": nil,
	} {
		loc, err := r.Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := Contained(loc); !errors.Is(err, want) && !(want == nil && err == nil) {
			t.Errorf("Contained(%q) = %v, want %v", p, err, want)
		}
	}
}

func TestMountForTurnsWindowsPathsIntoAPIPaths(t *testing.T) {
	if got := MountFor("/mnt/user"); got != "/mnt/user" {
		t.Errorf("MountFor(/mnt/user) = %q", got)
	}
	if filepath.Separator == '\\' {
		if got := MountFor(`D:\data\share`); got != "/D/data/share" {
			t.Errorf(`MountFor(D:\data\share) = %q`, got)
		}
	}
}

func FuzzResolveStaysInsideItsSource(f *testing.F) {
	for _, seed := range []string{"/", "/mnt/user/a", "/mnt/user/../x", "/mnt/user/a.zip::/b", "/mnt//user/./x", "/boot/..", "/mnt/userx", "/mnt/user::", "\x00"} {
		f.Add(seed)
	}
	r, err := New(nil,
		Source{Name: "Shares", Mount: "/mnt/user", Root: "/srv/user"},
		Source{Name: "USB-Stick", Mount: "/boot", Root: "/srv/boot"},
	)
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, p string) {
		loc, err := r.Resolve(p)
		if err != nil {
			return
		}
		if strings.Contains(loc.Path, "/../") || strings.HasSuffix(loc.Path, "/..") {
			t.Fatalf("Resolve(%q) kept a parent step: %q", p, loc.Path)
		}
		if loc.Source == nil {
			return
		}
		rel, err := filepath.Rel(loc.Source.Root, loc.Real)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("Resolve(%q) left %s: %q", p, loc.Source.Root, loc.Real)
		}
		again, err := Clean(loc.Path)
		if err != nil || again != loc.Path {
			t.Fatalf("Clean is not stable for %q: %q, %v", loc.Path, again, err)
		}
	})
}
