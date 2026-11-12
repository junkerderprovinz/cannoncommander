package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/modules"
)

func TestModulesMissingOrOlderThanTheProgramAreFetchedAgain(t *testing.T) {
	dir := t.TempDir()
	if !stale(dir, "v1.4.0") {
		t.Error("missing modules count as current")
	}
	write := func(release string) {
		rec, _ := json.Marshal(modules.Record{Release: release, Platform: runtime.GOOS + "/" + runtime.GOARCH})
		if err := os.WriteFile(filepath.Join(dir, modules.RecordFile), rec, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("v1.3.0")
	if !stale(dir, "v1.4.0") {
		t.Error("modules of 1.3.0 count as current for 1.4.0")
	}
	write("v1.4.0")
	if stale(dir, "v1.4.0") {
		t.Error("the modules of this version are fetched again")
	}
	write("v1.5.0")
	if stale(dir, "v1.4.0") {
		t.Error("newer modules are swapped for older ones")
	}
}
