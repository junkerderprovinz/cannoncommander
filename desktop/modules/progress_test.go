package modules

import (
	"strings"
	"testing"
)

func TestPrintProgressWritesTheLinesAnInstallerReads(t *testing.T) {
	var out strings.Builder
	report := PrintProgress(&out)
	report(0, "Downloading the modules")
	report(0.4567, "")
	report(1, "Done")
	want := "progress 0.00\nstatus Downloading the modules\nprogress 0.46\nprogress 1.00\nstatus Done\n"
	if out.String() != want {
		t.Errorf("wrote\n%s\nwant\n%s", out.String(), want)
	}
}
