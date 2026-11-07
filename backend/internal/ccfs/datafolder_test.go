package ccfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func dataFolderUser(root string) *users.User {
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	u := &users.User{BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: all}}}
	u.Username = "anna"
	return u
}

func writeAll(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func checkDataFolderIsClosed(t *testing.T, r *Resolver, u *users.User) {
	t.Helper()
	const db = "/mnt/user/appdata/cannoncommander/cannoncommander.sqlite"
	if _, err := r.Check(u, db, View|Download); !errors.Is(err, ErrNotFound) {
		t.Errorf("read: %v", err)
	}
	if _, err := r.Check(u, db, Modify); !errors.Is(err, ErrForbidden) {
		t.Errorf("write: %v", err)
	}
	if _, err := r.Check(u, "/mnt/user/appdata/cannoncommander", View); !errors.Is(err, ErrNotFound) {
		t.Errorf("folder: %v", err)
	}
	dir, err := r.Check(u, "/mnt/user/appdata", View)
	if err != nil {
		t.Fatal(err)
	}
	if r.Visible(u, dir, "cannoncommander") {
		t.Error("the folder shows up in its parent")
	}
	if !r.Visible(u, dir, "plex") {
		t.Error("a neighbour went away")
	}
	if err := r.Whole(context.Background(), u, dir, Delete); !errors.Is(err, ErrProtected) {
		t.Errorf("whole on the parent: %v", err)
	}
	if err := r.Carry(context.Background(), u, dir, Delete); !errors.Is(err, ErrProtected) {
		t.Errorf("carrying the parent: %v", err)
	}
	if _, err := r.Check(u, "/mnt/user/appdata/plex/config.xml", Modify); err != nil {
		t.Errorf("a neighbour is closed: %v", err)
	}
}

func TestTheDataFolderInsideASourceIsClosed(t *testing.T) {
	root := t.TempDir()
	writeAll(t, root, "appdata/cannoncommander/cannoncommander.sqlite", "appdata/plex/config.xml")
	r, err := New(nil, Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithDataFolder(filepath.Join(root, "appdata", "cannoncommander"))
	checkDataFolderIsClosed(t, r, dataFolderUser(root))
}

// A container sees its /config by another path than the share that holds it,
// which a link stands in for here.
func TestTheDataFolderSeenThroughASourceIsFoundByItsMarker(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	writeAll(t, root, "appdata/plex/config.xml")
	writeAll(t, data, "cannoncommander.sqlite")
	if err := os.Symlink(data, filepath.Join(root, "appdata", "cannoncommander")); err != nil {
		t.Skip("needs symlinks")
	}
	r, err := New(nil, Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithDataFolder(data)
	checkDataFolderIsClosed(t, r, dataFolderUser(root))
}

func TestADataFolderOutsideEverySourceClosesNothing(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	writeAll(t, root, "appdata/plex/config.xml")
	r, err := New(nil, Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithDataFolder(data)
	dir, err := r.Check(dataFolderUser(root), "/mnt/user/appdata", Delete)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Carry(context.Background(), dataFolderUser(root), dir, Delete); err != nil {
		t.Error(err)
	}
}

// mountinfo of a container on Unraid whose /config comes from an exclusive
// share on the cache pool, next to /mnt mapped as a whole.
const unraidMountinfo = `6504 6494 259:1 /test/cc\040l7/config /config rw,noatime shared:57 - xfs /dev/nvme1n1p1 rw
6505 6494 0:2 /mnt /mnt rw shared:58 master:1 - rootfs rootfs rw
6518 6505 259:1 / /mnt/cache rw,noatime shared:10 - xfs /dev/nvme1n1p1 rw
6519 6505 259:2 / /mnt/fastcache rw,noatime shared:11 - btrfs /dev/nvme0n1p1 rw
6567 6505 0:44 / /mnt/user0 rw,nosuid shared:12 - fuse.shfs shfs rw
6568 6505 0:45 / /mnt/user rw,nosuid shared:13 - fuse.shfs shfs rw
6569 6494 8:1 /config /boot/config ro shared:59 - vfat /dev/sda1 rw`

func TestTheMountTableTellsWhereElseTheDataFolderIs(t *testing.T) {
	for dir, want := range map[string][]string{
		"/config":          {"/mnt/cache/test/cc l7/config"},
		"/config/sessions": {"/mnt/cache/test/cc l7/config/sessions"},
		"/boot/config":     nil,
		"/srv/data":        nil,
	} {
		if got := mountedElsewhere(unraidMountinfo, dir); !slices.Equal(got, want) {
			t.Errorf("mountedElsewhere(%s) = %q, want %q", dir, got, want)
		}
	}
}

func TestTheDataFolderIsClosedInTheUserSharesAndOnEveryPool(t *testing.T) {
	mnt := unraidMnt(t)
	r := sharesAndRemotes(t, mnt, nil).WithDevices(mnt, "cache", "fastcache", "disk1")
	got := r.current().onEveryVolume([]string{"/mnt/cache/test/cc/config", "/remotes/x"})
	for _, want := range []string{"/mnt/user/test/cc/config", "/mnt/fastcache/test/cc/config", "/mnt/disk1/test/cc/config"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s is missing from %v", want, got)
		}
	}
	if slices.ContainsFunc(got, func(p string) bool { return strings.HasPrefix(p, "/remotes") }) {
		t.Errorf("a remote counts as a volume: %v", got)
	}
}
