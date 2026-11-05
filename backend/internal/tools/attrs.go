package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// KindAttrs sets mode and owner on folders and everything below them.
const KindAttrs = "attrsRecursive"

// StepLink is the undo step of a new link.
const StepLink = "link"

func init() {
	jobs.RegisterKind(KindAttrs, jobs.Kind{Prepare: prepareAttrs, Run: runAttrs})
	undo.Register(StepLink, undo.Reverser{Inline: undoLink})
}

// AttrsParams change mode, owner, group or the modification time. Fields
// left out stay as they are.
type AttrsParams struct {
	Paths []string `json:"paths"`
	// Mode is octal, as 0775 or 775.
	Mode  string     `json:"mode,omitempty"`
	Owner string     `json:"owner,omitempty"`
	Group string     `json:"group,omitempty"`
	MTime *time.Time `json:"mtime,omitempty"`
	// Recursive also changes everything inside folders and runs as a job.
	Recursive bool `json:"recursive,omitempty"`
}

type attrs struct {
	mode     fs.FileMode
	setMode  bool
	uid, gid int
	mtime    *time.Time
}

func (p AttrsParams) parse() (attrs, error) {
	a := attrs{uid: -1, gid: -1, mtime: p.MTime}
	if p.Mode != "" {
		m, err := strconv.ParseUint(p.Mode, 8, 32)
		if err != nil || m > 0o7777 {
			return a, badRequest("mode must be three or four octal digits")
		}
		a.mode, a.setMode = fs.FileMode(m&0o777), true
		if m&0o4000 != 0 {
			a.mode |= fs.ModeSetuid
		}
		if m&0o2000 != 0 {
			a.mode |= fs.ModeSetgid
		}
		if m&0o1000 != 0 {
			a.mode |= fs.ModeSticky
		}
	}
	var err error
	if p.Owner != "" {
		if a.uid, err = lookupID(p.Owner, false); err != nil {
			return a, err
		}
	}
	if p.Group != "" {
		if a.gid, err = lookupID(p.Group, true); err != nil {
			return a, err
		}
	}
	if !a.setMode && a.uid < 0 && a.gid < 0 && a.mtime == nil {
		return a, badRequest("nothing to change")
	}
	return a, nil
}

// lookupID accepts a name or a number, since the container may not know
// the names the host uses.
func lookupID(name string, group bool) (int, error) {
	if n, err := strconv.Atoi(name); err == nil && n >= 0 {
		return n, nil
	}
	var id string
	if group {
		g, err := user.LookupGroup(name)
		if err != nil {
			return -1, badRequest("there is no group " + name)
		}
		id = g.Gid
	} else {
		u, err := user.Lookup(name)
		if err != nil {
			return -1, badRequest("there is no user " + name)
		}
		id = u.Uid
	}
	return strconv.Atoi(id)
}

// mayUseSpecialBits keeps setuid and setgid for admins. The app often runs as
// root, so the bits would turn an uploaded program into a way up on the host.
func (p AttrsParams) mayUseSpecialBits(u *users.User) error {
	m, err := strconv.ParseUint(p.Mode, 8, 32)
	if err == nil && m&0o6000 != 0 && !u.Permissions.Admin {
		return ccfs.ErrForbidden
	}
	return nil
}

func (a attrs) apply(p string, info fs.FileInfo) error {
	link := info.Mode()&fs.ModeSymlink != 0
	if a.uid >= 0 || a.gid >= 0 {
		if err := os.Lchown(p, a.uid, a.gid); err != nil {
			return err
		}
	}
	// The mode of a link is meaningless, and chmod would change its target.
	if a.setMode && !link {
		if err := os.Chmod(p, a.mode); err != nil {
			return err
		}
	}
	if a.mtime != nil && !link {
		if err := os.Chtimes(p, *a.mtime, *a.mtime); err != nil {
			return err
		}
	}
	return nil
}

func (h handlers) setAttrs(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var p AttrsParams
	if err := v1.DecodeJSON(r, &p); err != nil {
		return err
	}
	if p.Recursive {
		raw, _ := json.Marshal(p)
		plan, err := prepareAttrs(r.Context(), h.deps, u, raw)
		if err != nil {
			return err
		}
		m := jobs.For(h.deps)
		if m == nil {
			return v1.Errorf(http.StatusServiceUnavailable, "the job queue is not running")
		}
		j, err := m.Submit(u.Username, KindAttrs, plan, 0, false)
		if err != nil {
			return err
		}
		return v1.WriteJSON(w, r, http.StatusAccepted, j)
	}
	if err := p.mayUseSpecialBits(u); err != nil {
		return err
	}
	a, err := p.parse()
	if err != nil {
		return err
	}
	if len(p.Paths) == 0 {
		return badRequest("paths are required")
	}
	var locs []ccfs.Location
	var infos []fs.FileInfo
	for _, s := range p.Paths {
		loc, info, err := entry(h.fs, u, s, ccfs.Modify)
		if err != nil {
			return err
		}
		locs, infos = append(locs, loc), append(infos, info)
	}
	for i, loc := range locs {
		if err := a.apply(loc.Real, infos[i]); err != nil {
			return err
		}
		fschange.Created(loc.Path)
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]int{"changed": len(locs)})
}

func prepareAttrs(ctx context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p AttrsParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if _, err := p.parse(); err != nil {
		return nil, err
	}
	if err := p.mayUseSpecialBits(u); err != nil {
		return nil, err
	}
	if len(p.Paths) == 0 {
		return nil, badRequest("paths are required")
	}
	plan := &jobs.Plan{}
	for i, s := range p.Paths {
		loc, _, err := entry(d.FS, u, s, ccfs.Modify)
		if err != nil {
			return nil, err
		}
		if err := d.FS.Whole(ctx, u, loc, ccfs.Modify); err != nil {
			return nil, err
		}
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: loc.Path, Name: path.Base(loc.Path)})
	}
	p.Recursive = true
	plan.Params = p
	return plan, nil
}

func runAttrs(ctx context.Context, run *jobs.Run) error {
	var p AttrsParams
	if err := run.Params(&p); err != nil {
		return err
	}
	a, err := p.parse()
	if err != nil {
		return err
	}
	for _, it := range pending(run) {
		loc, u, err := run.Entry(it.Path, ccfs.Modify)
		if err == nil {
			err = p.mayUseSpecialBits(u)
		}
		if err == nil {
			err = run.Deps().FS.Whole(ctx, u, loc, ccfs.Modify)
		}
		if err != nil {
			run.Finish(it.Seq, jobs.ItemFailed, err)
			continue
		}
		run.Begin(it.Seq)
		var count int64
		err = filepath.WalkDir(loc.Real, func(q string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && bookkeeping(d.Name()) {
				return filepath.SkipDir
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			run.Current(d.Name())
			if err := a.apply(q, info); err != nil {
				return err
			}
			count++
			return run.Advance(ctx, 0, 1)
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run.Measure(it.Seq, 0, count)
		run.Finish(it.Seq, jobs.ItemDone, err)
		fschange.Created(loc.Path)
	}
	return nil
}

// LinkRequest creates a symbolic link at Path pointing to Target, or with
// Hard a second name for the file Target.
type LinkRequest struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Hard   bool   `json:"hard,omitempty"`
}

type linkData struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Hard   bool   `json:"hard"`
}

var errLinkChanged = v1.Errorf(http.StatusConflict, "it changed since, so it is left as it is")

func (h handlers) link(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req LinkRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	at, err := h.fs.Check(u, req.Path, ccfs.Create)
	if err != nil {
		return err
	}
	if !ccfs.ValidName(path.Base(at.Path)) || at.Virtual() || at.InArchive() {
		return badRequest("invalid path")
	}
	if _, err := folder(h.fs, u, path.Dir(at.Path), ccfs.Create); err != nil {
		return err
	}
	if err := h.fs.Follows(u, at, ccfs.Create); err != nil {
		return err
	}
	if _, err := os.Lstat(at.Real); err == nil {
		return v1.Errorf(http.StatusConflict, "an entry with this name exists already")
	}
	clean, err := ccfs.Clean(req.Target)
	if err != nil {
		return badRequest("the target has to be a full path")
	}
	if clean == at.Path {
		return badRequest("a link cannot point to itself")
	}
	if req.Hard {
		// Writing to either name changes the one file, so the link takes the
		// right to change it.
		target, _, err := file(h.fs, u, clean, ccfs.View|ccfs.Download|ccfs.Modify)
		if err != nil {
			return err
		}
		if target.Source != at.Source {
			return badRequest("a hard link has to stay on the same source")
		}
		if err := os.Link(target.Real, at.Real); err != nil {
			if errors.Is(err, os.ErrExist) {
				return v1.Errorf(http.StatusConflict, "an entry with this name exists already")
			}
			return err
		}
	} else {
		// The target may not exist yet, but it has to be a place the user can
		// read. View alone also passes for the folders above their scope.
		target, err := h.fs.Reach(u, clean, ccfs.View|ccfs.Download)
		if err != nil {
			return err
		}
		if target.Virtual() || target.InArchive() {
			return badRequest("the target has to lie inside a source")
		}
		if err := os.Symlink(target.Real, at.Real); err != nil {
			return err
		}
	}
	fschange.Created(at.Path)
	out := map[string]string{"path": at.Path}
	if l := undo.For(h.deps); l != nil {
		step := undo.Step{Kind: StepLink, Name: path.Base(at.Path), Count: 1, Path: path.Dir(at.Path)}
		if s, err := l.Record(u.Username, step, linkData{Path: at.Path, Target: clean, Hard: req.Hard}); err == nil {
			out["undo"] = s.ID
		}
	}
	return v1.WriteJSON(w, r, http.StatusCreated, out)
}

func undoLink(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) error {
	var data linkData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	at, info, err := entry(d.FS, u, data.Path, ccfs.Delete)
	if err != nil {
		return err
	}
	if data.Hard {
		_, tinfo, err := entry(d.FS, u, data.Target, ccfs.View)
		if err != nil || !os.SameFile(info, tinfo) {
			return errLinkChanged
		}
	} else if info.Mode()&fs.ModeSymlink == 0 {
		return errLinkChanged
	}
	if err := os.Remove(at.Real); err != nil {
		return err
	}
	fschange.Removed(at.Path)
	return nil
}
