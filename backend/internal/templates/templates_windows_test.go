package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsOffersTheZipFolderOfItsShellNew(t *testing.T) {
	var zip *Template
	for _, x := range List() {
		if x.Ext == ".zip" {
			zip = &x
		}
		if x.Ext == ".lnk" {
			t.Error("the shortcut wizard is offered as a file")
		}
	}
	if zip == nil {
		t.Skip("this Windows registers no ZIP folder under New")
	}
	if zip.Name == "" || zip.File == "" {
		t.Errorf("zip = %+v", zip)
	}
	p := filepath.Join(t.TempDir(), "neu.zip")
	if err := Make(zip.ID, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); len(got) < 4 || string(got[:2]) != "PK" {
		t.Errorf("the new ZIP folder holds %q", got)
	}
}
