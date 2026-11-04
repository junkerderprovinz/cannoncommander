package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commonerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestShouldRefreshIndexAfterPut_SkipsWithoutIndexDB(t *testing.T) {
	indexing.SetTestIndex("Downloads", t.TempDir())
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})
	if shouldRefreshIndexAfterPut("Downloads") {
		t.Fatal("expected false when shared index DB is not initialized")
	}
}

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

func TestSanitizePutPathError_NoFilesystemPaths(t *testing.T) {
	_, idx := symlinkPutFixture(t)
	_, internalErr := idx.GetRealPathForMutation("", "/evil/hack.txt")
	if internalErr == nil {
		t.Fatal("expected resolution error")
	}

	status, clientErr := sanitizePutPathError(internalErr)
	if status != http.StatusForbidden {
		t.Fatalf("status=%d want 403", status)
	}
	if clientErr.Error() != "access denied" {
		t.Fatalf("message=%q want access denied", clientErr.Error())
	}
	if strings.Contains(clientErr.Error(), "outside") || strings.Contains(clientErr.Error(), "evil") {
		t.Fatalf("client error leaked path details: %q", clientErr.Error())
	}
}
