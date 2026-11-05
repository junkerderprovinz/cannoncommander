package accounts_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// openUpload makes an upload of owner that waits for its next chunk.
func (w *world) openUpload(owner string) (part string) {
	w.t.Helper()
	fileops.For(w.deps)
	w.t.Cleanup(func() {
		if m := jobs.For(w.deps); m != nil {
			m.Close()
		}
		if s := trash.For(w.deps); s != nil {
			s.Close()
		}
		for _, name := range []string{"jobs", "trash", "undo", "fileops"} {
			v1.Forget(w.deps, name)
		}
	})
	part = filepath.Join(w.share, "fotos", ".halb.bin.cc-part")
	if err := os.WriteFile(part, []byte("halb"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO uploads (id, owner, share, path, size, received, mtime, rule, created_at, updated_at)
		VALUES ('u1', ?, '', '/mnt/user/fotos/halb.bin', 10, 4, 0, 'replace', 1, 1)`, owner); err != nil {
		w.t.Fatal(err)
	}
	return part
}

func (w *world) uploadOwner() string {
	w.t.Helper()
	var owner string
	if err := w.deps.DB.QueryRow(`SELECT owner FROM uploads WHERE id = 'u1'`).Scan(&owner); err != nil {
		return ""
	}
	return owner
}

func TestANewUserOfAnOldNameFindsNoOpenUpload(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	part := w.openUpload("anna")

	w.must(http.StatusNoContent, w.as(w.admin.Username, "DELETE", "/admin/users/anna", nil))

	if got := w.uploadOwner(); got != "" {
		t.Errorf("the upload is still kept for %q", got)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Errorf("the part file stayed: %v", err)
	}
}

func TestRenamingAUserTakesOpenUploadsAlong(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.openUpload("anna")

	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"name": "anni"}))

	if got := w.uploadOwner(); got != "anni" {
		t.Errorf("the upload belongs to %q", got)
	}
}
