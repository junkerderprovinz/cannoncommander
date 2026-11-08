package places

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestAKnownFolderGoesByTheNameExplorerShows(t *testing.T) {
	e := newEnv(t, func(h string) []folder { return []folder{{"documents", filepath.Join(h, "Documents")}} })
	onDesktop(t, e.home)
	docs := filepath.Join(e.home, "Documents")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "desktop.ini"), []byte("[.ShellClassInfo]\r\nLocalizedResourceName=Dokumente\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := windows.UTF16PtrFromString(docs)
	if err := windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL) })

	_, _, body := e.do(http.MethodGet, "/places", nil)
	var got listJSON
	if err := json.Unmarshal(body, &got); err != nil || len(got.Folders) != 1 || got.Folders[0].Label != "Dokumente" {
		t.Fatalf("got %s, %v", body, err)
	}
}
