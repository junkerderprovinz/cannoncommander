package fileops_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// link gives the file at api a second name at to, as a hard link.
func (e *env) link(api, to string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.real(to)), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.Link(e.real(api), e.real(to)); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) sameFile(a, b string) bool {
	e.t.Helper()
	ia, err := os.Lstat(e.real(a))
	if err != nil {
		e.t.Fatal(err)
	}
	ib, err := os.Lstat(e.real(b))
	if err != nil {
		e.t.Fatal(err)
	}
	return os.SameFile(ia, ib)
}

func (e *env) trashOf(user string) []trash.Item {
	e.t.Helper()
	var out struct {
		Items []trash.Item `json:"items"`
	}
	e.call(user, "GET", "/trash", nil, &out)
	return out.Items
}

// sameReport reads how many files a copy or move left alone because the target
// was the same file.
func sameReport(t *testing.T, j jobs.Job) float64 {
	t.Helper()
	res, _ := j.Result.(map[string]any)
	n, _ := res["same"].(float64)
	return n
}

func TestCopyOrMoveOntoTheSameFileLeavesItAloneWhateverTheRule(t *testing.T) {
	for _, kind := range []string{"copy", "move"} {
		for _, rule := range []string{"replace", "both", "newer"} {
			t.Run(kind+" "+rule, func(t *testing.T) {
				e := newEnv(t)
				e.write("/mnt/user/fotos/a.jpg", "sonne", old)
				e.link("/mnt/user/fotos/a.jpg", "/mnt/user/backups/a.jpg")

				j := e.run(kind, fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.jpg"}, Dest: "/mnt/user/backups", Rule: rule})
				if !e.sameFile("/mnt/user/fotos/a.jpg", "/mnt/user/backups/a.jpg") || e.read("/mnt/user/backups/a.jpg") != "sonne" {
					t.Fatal("the file lost one of its names")
				}
				if items := e.trashItems(); len(items) != 0 {
					t.Fatalf("trash holds %+v", items)
				}
				if !e.gone("/mnt/user/backups/a (1).jpg") {
					t.Fatal("a second copy was made next to the file")
				}
				if len(j.Items) != 1 || j.Items[0].State != jobs.ItemSkipped {
					t.Fatalf("items %+v", j.Items)
				}
				if sameReport(t, j) != 1 {
					t.Fatalf("result %v", j.Result)
				}
			})
		}
	}
}

func TestAFolderGoesOverExceptTheFilesThatAreTheSame(t *testing.T) {
	for _, kind := range []string{"copy", "move"} {
		t.Run(kind, func(t *testing.T) {
			e := newEnv(t)
			e.write("/mnt/user/fotos/Urlaub/a.jpg", "sonne", old)
			e.write("/mnt/user/fotos/Urlaub/b.jpg", "strand", old)
			e.link("/mnt/user/fotos/Urlaub/a.jpg", "/mnt/user/backups/Urlaub/a.jpg")

			j := e.run(kind, fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups", Rule: "replace"})
			if !e.sameFile("/mnt/user/fotos/Urlaub/a.jpg", "/mnt/user/backups/Urlaub/a.jpg") {
				t.Fatal("the shared file lost a name")
			}
			if e.read("/mnt/user/backups/Urlaub/b.jpg") != "strand" {
				t.Fatal("the other file did not arrive")
			}
			if kind == "move" && !e.gone("/mnt/user/fotos/Urlaub/b.jpg") {
				t.Fatal("the moved file stayed at the source")
			}
			if items := e.trashItems(); len(items) != 0 {
				t.Fatalf("trash holds %+v", items)
			}
			if sameReport(t, j) != 1 {
				t.Fatalf("result %v", j.Result)
			}
		})
	}
}

// The user share is a FUSE file system on Unraid, which shows its own device
// and inode for a file that lies on a disk. Here the user share is a plain
// folder that holds a separate file at the same path, so only the way through
// Unraid's disks can tell that both are one.
func TestTheDiskShareAndTheUserShareAboveItAreTheSameFile(t *testing.T) {
	for _, kind := range []string{"copy", "move"} {
		t.Run(kind, func(t *testing.T) {
			a := newArray(t)
			a.write(a.path("disk1", "filme/a.mkv"), "sintel")
			e := newEnv(t, ccfs.Source{Name: "Disk 1", Mount: "/mnt/disk1", Root: a.path("disk1", "")})
			e.write("/mnt/user/filme/a.mkv", "sintel", old)

			var j jobs.Job
			body := map[string]any{"kind": kind, "params": fileops.TransferParams{Sources: []string{"/mnt/disk1/filme/a.mkv"}, Dest: "/mnt/user/filme", Rule: "replace"}}
			if code := e.call("admin", "POST", "/jobs", body, &j); code != http.StatusAccepted {
				t.Fatalf("start: %d", code)
			}
			j = waitAs(e, "admin", j.ID)
			if j.State != jobs.Done {
				t.Fatalf("job ended %s: %s %+v", j.State, j.Error, j.Items)
			}
			if !a.has("disk1", "filme/a.mkv") {
				t.Fatal("the file left its disk")
			}
			if items := e.trashOf("admin"); len(items) != 0 {
				t.Fatalf("trash holds %+v", items)
			}
			if len(j.Items) != 1 || j.Items[0].State != jobs.ItemSkipped || sameReport(t, j) != 1 {
				t.Fatalf("items %+v, result %v", j.Items, j.Result)
			}
		})
	}
}

func TestPreflightTellsTheSameFileFromAConflict(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "sonne", old)
	e.write("/mnt/user/fotos/b.jpg", "strand", old)
	e.link("/mnt/user/fotos/a.jpg", "/mnt/user/backups/a.jpg")
	e.write("/mnt/user/backups/b.jpg", "regen", old)
	var p fileops.Preflight
	body := map[string]any{"kind": "copy", "sources": []string{"/mnt/user/fotos/a.jpg", "/mnt/user/fotos/b.jpg"}, "dest": "/mnt/user/backups"}
	if code := e.call("anna", "POST", "/ops/preflight", body, &p); code != http.StatusOK {
		t.Fatalf("preflight: %d", code)
	}
	if len(p.Same) != 1 || p.Same[0] != "a.jpg" {
		t.Errorf("same %v", p.Same)
	}
	if len(p.Conflicts) != 1 || p.Conflicts[0].Key != "b.jpg" {
		t.Errorf("conflicts %+v", p.Conflicts)
	}
}

func TestPreflightSeesTheDiskShareUnderTheUserShare(t *testing.T) {
	a := newArray(t)
	a.write(a.path("disk1", "filme/a.mkv"), "sintel")
	e := newEnv(t, ccfs.Source{Name: "Disk 1", Mount: "/mnt/disk1", Root: a.path("disk1", "")})
	e.write("/mnt/user/filme/a.mkv", "sintel", old)
	var p fileops.Preflight
	body := map[string]any{"kind": "move", "sources": []string{"/mnt/disk1/filme/a.mkv"}, "dest": "/mnt/user/filme"}
	if code := e.call("admin", "POST", "/ops/preflight", body, &p); code != http.StatusOK {
		t.Fatalf("preflight: %d", code)
	}
	if len(p.Same) != 1 || len(p.Conflicts) != 0 {
		t.Errorf("same %v, conflicts %+v", p.Same, p.Conflicts)
	}
}
