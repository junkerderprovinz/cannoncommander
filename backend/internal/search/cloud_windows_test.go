package search

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"golang.org/x/sys/windows"
)

func TestTheTextSearchLeavesAFileInTheCloudUnread(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notiz.txt")
	put(t, p, "Router im Keller", 0)
	name, _ := windows.UTF16PtrFromString(p)
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_OFFLINE); err != nil {
		t.Fatal(err)
	}
	if _, found, err := findIn(context.Background(), p, []byte("router")); found || !errors.Is(err, ccfs.ErrInCloud) {
		t.Errorf("found %v, err %v", found, err)
	}
}
