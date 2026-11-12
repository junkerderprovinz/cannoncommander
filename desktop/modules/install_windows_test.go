package modules

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A program that runs from the modules folder holds it, as an open file
// does here, and Windows will not move the folder until it ends.
func TestModulesInUseWaitForTheNextStart(t *testing.T) {
	dir := oldModules(t)
	held, err := os.Open(filepath.Join(dir, "tool", "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	u := release(t, "v1.4.0", map[string][]byte{asset: modulesZip(t, "new tool")}, nil)
	err = (&Installer{Updater: u, Dir: dir}).Install(context.Background(), "v1.4.0")
	if !errors.Is(err, ErrNextStart) {
		held.Close()
		t.Fatalf("got %v, want ErrNextStart", err)
	}
	if rec, err := Read(dir); err != nil || rec.Release != "v1.3.0" {
		t.Errorf("the running modules changed: %+v, %v", rec, err)
	}
	held.Close()

	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "tool", "bin", "tool")); string(got) != "new tool" {
		t.Errorf("after the next start the tool is %q", got)
	}
	if _, err := os.Stat(dir + ".next"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the waiting modules stayed: %v", err)
	}
}
