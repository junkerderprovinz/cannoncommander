package web

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	commonerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func symlinkPutFixture(t *testing.T) (downloadsPath string, idx *indexing.Index) {
	t.Helper()
	downloadsPath = filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(filepath.Join(downloadsPath, "projects"), 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(downloadsPath), "outside-target")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(downloadsPath, "evil")); err != nil {
		t.Fatal(err)
	}
	idx = &indexing.Index{Source: settings.Source{Path: downloadsPath}}
	return downloadsPath, idx
}

func TestResolvePutRealPath_NewFileViaParent(t *testing.T) {
	_, idx := symlinkPutFixture(t)

	realPath, err := idx.GetRealPathForMutation("", "/projects/newfile.txt")
	if err != nil {
		t.Fatalf("GetRealPathForMutation: %v", err)
	}
	parentReal, isDir, err := idx.GetRealPath("/projects")
	if err != nil || !isDir {
		t.Fatalf("parent dir: err=%v isDir=%v", err, isDir)
	}
	want := filepath.Join(parentReal, "newfile.txt")
	if realPath != want {
		t.Fatalf("got %q want %q", realPath, want)
	}
}

func TestResolvePutRealPath_RejectsEscapingParent(t *testing.T) {
	_, idx := symlinkPutFixture(t)

	_, err := idx.GetRealPathForMutation("", "/evil/hack.txt")
	if !errors.Is(err, commonerrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope, got %v", err)
	}
}
