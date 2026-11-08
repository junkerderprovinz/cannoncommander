package tools

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestTheChecksumLeavesAFileInTheCloudUnread(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/wolke.jpg", "fern")
	e.write("/mnt/user/fotos/hier.jpg", "nah")
	name, _ := windows.UTF16PtrFromString(e.real("/mnt/user/fotos/wolke.jpg"))
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_OFFLINE); err != nil {
		t.Fatal(err)
	}
	j := e.finish(KindChecksum, ChecksumParams{Sources: []string{"/mnt/user/fotos"}, Algo: "sha256"})
	if len(j.Items) != 1 || j.Items[0].ErrorCode != "inCloud" {
		t.Errorf("items %+v", j.Items)
	}
	sums := result[struct {
		Files []FileSum `json:"files"`
	}](t, j).Files
	got := map[string]string{}
	for _, s := range sums {
		got[s.Path] = s.Hash
	}
	if got["/mnt/user/fotos/hier.jpg"] == "" {
		t.Errorf("the local file has no sum: %+v", sums)
	}
	if got["/mnt/user/fotos/wolke.jpg"] != "" {
		t.Errorf("the file in the cloud was read: %+v", sums)
	}
}
