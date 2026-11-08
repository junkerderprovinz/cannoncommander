package ccfs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAnOfflineFileCountsAsInTheCloud(t *testing.T) {
	dir := t.TempDir()
	online, local := filepath.Join(dir, "online.jpg"), filepath.Join(dir, "local.jpg")
	for _, p := range []string{online, local} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	name, _ := windows.UTF16PtrFromString(online)
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_OFFLINE); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(online)
	if err != nil {
		t.Fatal(err)
	}
	if !InCloud(online) || !InCloudInfo(info) {
		t.Error("the offline file reads as local")
	}
	if InCloud(local) {
		t.Error("the local file reads as online")
	}
}
