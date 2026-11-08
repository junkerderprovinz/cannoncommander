package archives

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestFindsSevenZipWhereItsInstallerPutIt(t *testing.T) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\7-Zip`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		t.Skip("7-Zip is not installed here")
	}
	dir, _, err := k.GetStringValue("Path")
	k.Close()
	if _, serr := os.Stat(filepath.Join(dir, "7z.exe")); err != nil || serr != nil {
		t.Skip("7-Zip names no folder that holds it")
	}
	t.Setenv("PATH", t.TempDir())
	if got := Find("7zz", "7z"); !strings.EqualFold(got, filepath.Join(dir, "7z.exe")) {
		t.Errorf("Find = %q, want 7z.exe in %s", got, dir)
	}
}
