package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// privateBus starts a session bus of the test's own.
func privateBus(t *testing.T) string {
	t.Helper()
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(addr)
}

func connect(t *testing.T, addr string) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestOtherProgramsShowFoldersAndFilesHere(t *testing.T) {
	addr := privateBus(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "download.zip")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	shown := make(chan []opening, 4)
	if err := serveFolders(connect(t, addr), func(o []opening) { shown <- o }); err != nil {
		t.Fatal(err)
	}
	browser := connect(t, addr).Object(fileManager1, fileManager1Path)
	calls := []struct {
		method string
		want   []opening
	}{
		{"ShowItems", []opening{{Folder: dir, Select: "download.zip"}}},
		{"ShowFolders", []opening{{Folder: dir}}},
	}
	for _, c := range calls {
		uri := fileURI(file)
		if c.method == "ShowFolders" {
			uri = fileURI(dir)
		}
		if err := browser.Call(fileManager1+"."+c.method, 0, []string{uri}, "").Err; err != nil {
			t.Fatalf("%s: %v", c.method, err)
		}
		select {
		case got := <-shown:
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("%s: got %+v", c.method, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s showed nothing", c.method)
		}
	}
}
