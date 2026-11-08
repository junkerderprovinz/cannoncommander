package tools

import (
	"net/http"
	"runtime"
	"testing"
)

func TestWindowsAttributesTakeOnlyTheKnownNames(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "a")
	bad := AttrsParams{Paths: []string{"/mnt/user/fotos/a.txt"}, Flags: map[string]bool{"compressed": true}}
	if code := e.call("anna", "PATCH", "/fs/attrs", bad, nil); code != http.StatusBadRequest {
		t.Errorf("unknown attribute %d", code)
	}
	if runtime.GOOS == "windows" {
		return
	}
	p := AttrsParams{Paths: []string{"/mnt/user/fotos/a.txt"}, Flags: map[string]bool{"readOnly": true}}
	if code := e.call("anna", "PATCH", "/fs/attrs", p, nil); code != http.StatusBadRequest {
		t.Errorf("attributes of Windows elsewhere %d", code)
	}
}

func TestKeepingAFileOnTheDeviceAndFreeingItAreOneSwitch(t *testing.T) {
	if got := withFlags(attrUnpinned|attrArchive, map[string]bool{"pinned": true}); got != attrPinned|attrArchive {
		t.Errorf("keep = %#x", got)
	}
	if got := withFlags(attrPinned, map[string]bool{"pinned": false, "readOnly": true}); got != attrUnpinned|attrReadOnly {
		t.Errorf("free = %#x", got)
	}
}
