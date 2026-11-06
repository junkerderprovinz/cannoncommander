package tools

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// partSuffix marks a file that is still being written, as the copy jobs do.
const partSuffix = ccfs.PartSuffix

func partOf(p string) string {
	return path.Join(path.Dir(p), "."+path.Base(p)+partSuffix)
}

// entry checks that p is an existing file or folder inside a source and
// that u holds perm there.
func entry(fsys *ccfs.Resolver, u *users.User, p string, perm ccfs.Perm) (ccfs.Location, os.FileInfo, error) {
	loc, err := fsys.Entry(u, p, perm)
	if err != nil {
		return loc, nil, err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
		return loc, nil, v1.Errorf(http.StatusBadRequest, "only files and folders inside a source can be used here")
	}
	info, err := os.Lstat(loc.Real)
	return loc, info, err
}

// file is entry for a regular file.
func file(fsys *ccfs.Resolver, u *users.User, p string, perm ccfs.Perm) (ccfs.Location, os.FileInfo, error) {
	loc, info, err := entry(fsys, u, p, perm)
	if err == nil && !info.Mode().IsRegular() {
		err = v1.Errorf(http.StatusBadRequest, p+" is not a regular file")
	}
	return loc, info, err
}

// folder checks that p is a folder inside a source where u holds perm.
func folder(fsys *ccfs.Resolver, u *users.User, p string, perm ccfs.Perm) (ccfs.Location, error) {
	loc, err := fsys.Landed(u, p, perm)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() {
		return loc, v1.Coded(http.StatusBadRequest, "outsideSources", "the target has to be a folder inside a source")
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return loc, err
	}
	if !info.IsDir() {
		return loc, v1.Errorf(http.StatusBadRequest, p+" is not a folder")
	}
	return loc, nil
}

func badRequest(msg string) error { return v1.Errorf(http.StatusBadRequest, msg) }

// pending returns the items of a run that still have to be done.
func pending(run *jobs.Run) []jobs.Item {
	var out []jobs.Item
	for _, it := range run.Items() {
		if it.State == jobs.ItemPending || it.State == jobs.ItemRunning {
			out = append(out, it)
		}
	}
	return out
}

// progress turns written bytes into the job's progress and pause points.
func progress(ctx context.Context, run *jobs.Run) func(int) error {
	return func(n int) error { return run.Transfer(ctx, n) }
}

// copyCounted copies r into w in chunks that count as the job's progress.
func copyCounted(ctx context.Context, run *jobs.Run, w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, jobs.Chunk)
	var total int64
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return total, err
			}
			total += int64(n)
			if err := run.Transfer(ctx, n); err != nil {
				return total, err
			}
		}
		if errors.Is(rerr, io.EOF) {
			return total, nil
		}
		if rerr != nil {
			return total, rerr
		}
	}
}

// bookkeeping names entries CannonCommander keeps for itself.
func bookkeeping(name string) bool {
	return name == trash.DirName || strings.HasSuffix(name, partSuffix)
}

// secrets keeps archive passwords out of the job table. A job only stores
// a key to its password, and a password lives until the server stops, so
// a job resumed after a restart asks for it again.
var secrets sync.Map

func keepSecret(pw string) string {
	if pw == "" {
		return ""
	}
	key := rand.Text()
	secrets.Store(key, pw)
	return key
}

func secret(key string) string {
	if key == "" {
		return ""
	}
	v, _ := secrets.Load(key)
	s, _ := v.(string)
	return s
}

func forgetSecret(key string) {
	if key != "" {
		secrets.Delete(key)
	}
}

// errPasswordGone ends a job resumed after a restart, whose password the
// server forgot when it stopped.
var errPasswordGone = errors.New("the password was only kept until the server restarted, start the job again")

// sameDir reports the folder all paths lie in, or "" when they differ.
func sameDir(paths []string) string {
	dir := ""
	for i, p := range paths {
		d := path.Dir(p)
		if i == 0 {
			dir = d
		} else if d != dir {
			return ""
		}
	}
	return dir
}

func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
