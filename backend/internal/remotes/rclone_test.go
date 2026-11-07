package remotes

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// realService runs the service against rclone itself, with a local folder
// as the remote. It needs rclone on the PATH.
func realService(t *testing.T) (*Service, string) {
	t.Helper()
	bin := system.RcloneBinary()
	if bin == "" {
		t.Skip("rclone is not installed")
	}
	base := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(base, "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(base, "data")
	if err := os.MkdirAll(filepath.Join(data, "Fotos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "notiz.txt"), []byte("hallo"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Binary: bin,
		Dir:    filepath.Join(base, "config", "rclone"),
		Socket: filepath.Join(base, "rc", "rclone.sock"),
		Root:   filepath.Join(base, "remotes"),
	}
	mounting := func(context.Context) (system.Mounting, error) {
		return system.Mounting{Automount: true, Cache: system.CacheWrites, CacheSize: 1}, nil
	}
	s := newService(cfg, store{db}, mounting)
	t.Cleanup(s.Stop)
	return s, data
}

func canMount() bool {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return false
	}
	if _, err := os.Stat("/dev/fuse"); err != nil {
		return false
	}
	_, err := exec.LookPath("fusermount3")
	return err == nil
}

func TestRcloneCreatesTestsAndForgetsALocalRemote(t *testing.T) {
	s, data := realService(t)
	ctx := context.Background()

	types, err := s.Types(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) < 20 || types[0].ID != "sftp" || types[4].ID != "drive" || !types[4].OAuth {
		t.Fatalf("types start with %+v", types[:5])
	}

	res, err := s.Test(ctx, Input{Type: "local", Path: &data})
	if err != nil || !res.OK || res.Entries != 2 {
		t.Fatalf("test: %+v %v", res, err)
	}
	missing := filepath.Join(data, "nope")
	if res, err := s.Test(ctx, Input{Type: "local", Path: &missing}); err != nil || res.OK || res.Error == "" {
		t.Fatalf("test of a missing folder: %+v %v", res, err)
	}

	r, err := s.Create(ctx, Input{Name: "nas", Type: "sftp", Params: map[string]string{"host": "nas.invalid", "user": "jdp", "pass": "geheim"}})
	if err != nil {
		t.Fatal(err)
	}
	sec, err := s.section(ctx, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if sec["pass"] == "" || sec["pass"] == "geheim" || r.Params["pass"] != "" {
		t.Fatalf("password stored as %q, shown as %q", sec["pass"], r.Params["pass"])
	}
	if _, err := s.Update(ctx, "nas", Input{Name: "nas2", Params: map[string]string{"host": "nas2.invalid", "pass": ""}}); err != nil {
		t.Fatal(err)
	}
	moved, err := s.section(ctx, "nas2")
	if err != nil || moved["pass"] != sec["pass"] || moved["host"] != "nas2.invalid" {
		t.Fatalf("moved %v %v", moved, err)
	}
	if err := s.Delete(ctx, "nas2"); err != nil {
		t.Fatal(err)
	}
	var names struct {
		Remotes []string `json:"remotes"`
	}
	if err := s.rc.call(ctx, "config/listremotes", nil, &names); err != nil || len(names.Remotes) != 0 {
		t.Fatalf("left in rclone.conf: %v %v", names.Remotes, err)
	}
}

func TestRcloneMountsAndComesBackAfterACrash(t *testing.T) {
	if !canMount() {
		t.Skip("mounting needs root, /dev/fuse and fusermount3")
	}
	s, data := realService(t)
	ctx := context.Background()
	r, err := s.Create(ctx, Input{Name: "lokal", Type: "local", Path: &data})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Mounted || r.Error != "" {
		t.Fatalf("not mounted: %+v", r)
	}
	got, err := os.ReadFile(filepath.Join(r.Mount, "notiz.txt"))
	if err != nil || string(got) != "hallo" {
		t.Fatalf("read through the mount: %q %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(r.Mount, "neu.txt"), []byte("neu"), 0o644); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the written file reaches the remote", func() bool {
		b, err := os.ReadFile(filepath.Join(data, "neu.txt"))
		return err == nil && string(b) == "neu"
	})

	// rcd ends, the supervisor starts it again and mounts what was mounted.
	_ = s.rc.call(ctx, "core/quit", nil, nil)
	eventually(t, "rcd is down", func() bool { return !s.status().Running })
	eventually(t, "the mount is back", func() bool {
		_, err := os.Stat(filepath.Join(r.Mount, "notiz.txt"))
		return s.status().Running && s.mountedPoints(ctx)[r.Mount] && err == nil
	})

	if _, err := s.Unmount(ctx, "lokal"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(r.Mount); len(entries) != 0 {
		t.Fatalf("still mounted: %v", entries)
	}
	if _, err := s.Mount(ctx, "lokal"); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if _, err := os.Stat(filepath.Join(r.Mount, "notiz.txt")); err == nil {
		t.Fatal("the mount outlived rcd")
	}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("waited in vain until %s", what)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
