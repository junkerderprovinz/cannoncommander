package main

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// fileURI is the URI a browser or a file manager hands over for p.
func fileURI(p string) string {
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return (&url.URL{Scheme: "file", Path: s}).String()
}

func TestFoldersAndFilesHandedOverOpenAsFolders(t *testing.T) {
	dir := t.TempDir()
	photos := filepath.Join(dir, "Fotos mit Leerzeichen")
	if err := os.Mkdir(photos, 0o755); err != nil {
		t.Fatal(err)
	}
	pic := filepath.Join(photos, "see.jpg")
	if err := os.WriteFile(pic, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, others := handedOver([]string{photos, fileURI(pic), "Fotos mit Leerzeichen", filepath.Join(dir, "gone"), "--home"}, dir)
	want := []opening{{Folder: photos}, {Folder: photos, Select: "see.jpg"}, {Folder: photos}, {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if len(others) != 0 {
		t.Fatalf("passed on %v", others)
	}
}

func TestShellPlacesGoToExplorer(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows hands over shell places")
	}
	got, others := handedOver([]string{"::{20D04FE0-3AEA-1069-A2D8-08002B30309D}", "shell:Downloads"}, `C:\`)
	if len(got) != 0 || !reflect.DeepEqual(others, []string{"::{20D04FE0-3AEA-1069-A2D8-08002B30309D}", "shell:Downloads"}) {
		t.Fatalf("got %+v, passed on %v", got, others)
	}
}

// A quoted drive root, "D:\", reaches a program as D:" because the parser
// reads \" as a quote, as Windows passed it from older registrations.
func TestADriveRootOpensAlsoWhenItsQuoteWasEscaped(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows hands over drive roots")
	}
	drive := filepath.VolumeName(t.TempDir())
	got, _ := handedOver([]string{drive + `"`, drive + `\`}, `C:\`)
	want := []opening{{Folder: drive + `\`}, {Folder: drive + `\`}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
