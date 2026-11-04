//go:build unix

package fileops_test

import (
	"os"
	"testing"
)

func TestGatherLeavesHardLinkedFilesWhereTheyAre(t *testing.T) {
	e := newEnv(t)
	a := newArray(t)
	a.write(a.path("disk1", "filme/Doppelt/a.mkv"), "linked")
	if err := os.Link(a.path("disk1", "filme/Doppelt/a.mkv"), a.path("disk1", "filme/Doppelt/b.mkv")); err != nil {
		t.Fatal(err)
	}
	a.write(a.path("disk1", "filme/Sintel.mkv"), "sintel")
	j := gather(e, "disk2")
	if !a.has("disk1", "filme/Doppelt/a.mkv") || !a.has("disk1", "filme/Doppelt/b.mkv") || a.has("disk2", "filme/Doppelt") {
		t.Error("hard linked files have to stay together on disk1")
	}
	if res, _ := j.Result.(map[string]any); res["moved"] != float64(1) || res["linked"] != float64(2) {
		t.Errorf("result = %v", j.Result)
	}
}
