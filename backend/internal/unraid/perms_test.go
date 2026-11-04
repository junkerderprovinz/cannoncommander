//go:build unix

package unraid

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func permsFixture(t *testing.T) (*env, *ccfs.Resolver, *users.User) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("changing owners needs root")
	}
	e := newEnv(t)
	e.write(filepath.Join(e.cfg.BootConfig, "domain.cfg"), "DOMAINDIR=\"/mnt/user/vms/\"\n")
	user := filepath.Join(e.cfg.Mnt, "user")
	e.file("user", "medien/Filme/a.mkv", 1)
	e.file("user", "medien/appdata/ist-kein-share.txt", 1)
	e.file("user", "appdata/plex/db.sqlite", 1)
	e.file("user", "vms/win.img", 1)
	if err := os.Symlink("/etc/passwd", filepath.Join(user, "medien", "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(user, "medien", "Filme", "a.mkv"), 0o4755); err != nil {
		t.Fatal(err)
	}
	r, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: user})
	if err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}
	admin := &users.User{BackendScopes: []users.BackendScope{{Path: user, Scope: "/", Permissions: all}}}
	admin.Username, admin.Permissions.Admin = "jdp", true
	return e, r, admin
}

func ownerAndMode(t *testing.T, p string) (uint32, uint32, os.FileMode) {
	t.Helper()
	info, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	return st.Uid, st.Gid, info.Mode() & (os.ModePerm | os.ModeSetuid)
}

func TestFixPermsSetsNobodyUsersAndKeepsContainerFolders(t *testing.T) {
	e, r, admin := permsFixture(t)
	loc, err := r.Check(admin, "/mnt/user", ccfs.View)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := e.svc.ScanPerms(context.Background(), loc)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Blocked != "" || scan.Files != 3 || scan.Dirs != 3 || scan.Wrong == 0 || len(scan.Kept) != 2 {
		t.Fatalf("scan = %+v", scan)
	}
	if scan.Kept[0] != (KeptPath{Path: "/mnt/user/appdata", Reason: KeepAppdata, Size: 7 * 1024}) || scan.Kept[1].Path != "/mnt/user/vms" || scan.Kept[1].Reason != KeepDomains {
		t.Errorf("kept = %+v", scan.Kept)
	}

	task, err := e.svc.PreparePerms(r, admin, "/mnt/user")
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	res, err := task.Run(context.Background(), func(p string) error {
		seen = append(seen, p)
		return nil
	})
	if err != nil || res.Failed != 0 || res.Changed == 0 || len(res.Kept) != 2 {
		t.Fatalf("run = %+v, %v", res, err)
	}
	user := filepath.Join(e.cfg.Mnt, "user")
	for p, want := range map[string]os.FileMode{
		"medien":                            0o777,
		"medien/Filme":                      0o777,
		"medien/Filme/a.mkv":                0o666,
		"medien/appdata/ist-kein-share.txt": 0o666,
	} {
		uid, gid, mode := ownerAndMode(t, filepath.Join(user, p))
		if uid != 99 || gid != 100 || mode != want {
			t.Errorf("%s: %d:%d %o, want 99:100 %o", p, uid, gid, mode, want)
		}
	}
	if uid, _, _ := ownerAndMode(t, filepath.Join(user, "appdata", "plex", "db.sqlite")); uid == 99 {
		t.Error("appdata got new owners")
	}
	if uid, _, _ := ownerAndMode(t, "/etc/passwd"); uid != 0 {
		t.Error("the run followed a link out of the folder")
	}
	if uid, _, _ := ownerAndMode(t, filepath.Join(user, "medien", "link")); uid != 99 {
		t.Error("the link itself kept its owner")
	}
	if len(seen) != res.Entries {
		t.Errorf("step saw %d entries, run counted %d", len(seen), res.Entries)
	}

	again, _ := e.svc.ScanPerms(context.Background(), loc)
	if again.Wrong != 0 {
		t.Errorf("after the run %d entries are still wrong", again.Wrong)
	}
}

func TestFixPermsStopsWhenTheStepSaysSo(t *testing.T) {
	e, r, admin := permsFixture(t)
	task, err := e.svc.PreparePerms(r, admin, "/mnt/user/medien")
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("cancelled")
	n := 0
	_, err = task.Run(context.Background(), func(string) error {
		if n++; n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Errorf("err = %v", err)
	}
}

func TestFixPermsRefusesKeptFoldersAndNonAdmins(t *testing.T) {
	e, r, admin := permsFixture(t)
	status := func(err error) int {
		var p *v1.Problem
		if errors.As(err, &p) {
			return p.Status
		}
		return 0
	}
	if _, err := e.svc.PreparePerms(r, admin, "/mnt/user/appdata/plex"); status(err) != http.StatusConflict {
		t.Errorf("appdata: %v", err)
	}
	if _, err := e.svc.PreparePerms(r, admin, "/mnt/user/medien/Filme/a.mkv"); status(err) != http.StatusBadRequest {
		t.Errorf("a file: %v", err)
	}
	someone := *admin
	someone.Permissions.Admin = false
	if _, err := e.svc.PreparePerms(r, &someone, "/mnt/user/medien"); status(err) != http.StatusForbidden {
		t.Errorf("non admin: %v", err)
	}
	loc, _ := r.Check(admin, "/mnt/user/vms", ccfs.View)
	if scan, _ := e.svc.ScanPerms(context.Background(), loc); scan.Blocked != KeepDomains {
		t.Errorf("scan of vms = %+v", scan)
	}
}
