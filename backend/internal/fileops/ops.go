package fileops

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"io/fs"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// Kinds of undo steps besides copy and move.
const (
	StepRename = "rename"
	StepMkdir  = "mkdir"
	StepTouch  = "touch"
)

var errExists = v1.Coded(http.StatusConflict, "nameTaken", "an entry with this name exists already")

// Changed is the answer of the operations that create or rename.
type Changed struct {
	Path    string `json:"path"`
	Created bool   `json:"created,omitempty"`
	Undo    string `json:"undo,omitempty"`
}

type pathRequest struct {
	Path string `json:"path"`
}

func (s *Service) mkdir(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req pathRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	loc, err := s.newEntry(u, req.Path)
	if err != nil {
		return err
	}
	if err := os.Mkdir(loc.Real, 0o777); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errExists
		}
		return err
	}
	fschange.Created(loc.Path)
	out := Changed{Path: loc.Path, Created: true}
	out.Undo = s.record(u, undo.Step{Kind: StepMkdir, Name: path.Base(loc.Path), Count: 1, Path: path.Dir(loc.Path)}, entryData{Path: loc.Path})
	return v1.WriteJSON(w, r, http.StatusCreated, out)
}

type touchRequest struct {
	Path  string     `json:"path"`
	MTime *time.Time `json:"mtime"`
}

func (s *Service) touch(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req touchRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	when := time.Now()
	if req.MTime != nil {
		when = *req.MTime
	}
	if loc, err := s.fs.Check(u, req.Path, ccfs.Modify); err == nil && !loc.Virtual() {
		if _, err := os.Lstat(loc.Real); err == nil {
			if err := s.fs.Follows(u, loc, ccfs.Modify); err != nil {
				return err
			}
			if err := os.Chtimes(loc.Real, when, when); err != nil {
				return err
			}
			return v1.WriteJSON(w, r, http.StatusOK, Changed{Path: loc.Path})
		}
	}
	loc, err := s.newEntry(u, req.Path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(loc.Real, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errExists
		}
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if req.MTime != nil {
		if err := os.Chtimes(loc.Real, when, when); err != nil {
			return err
		}
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	fschange.Created(loc.Path)
	out := Changed{Path: loc.Path, Created: true}
	out.Undo = s.record(u, undo.Step{Kind: StepTouch, Name: path.Base(loc.Path), Count: 1, Path: path.Dir(loc.Path)},
		entryData{Path: loc.Path, MTime: info.ModTime()})
	return v1.WriteJSON(w, r, http.StatusCreated, out)
}

type renameRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (s *Service) rename(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req renameRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if !ccfs.ValidName(req.Name) {
		return v1.Coded(http.StatusBadRequest, "badName", "invalid name")
	}
	from, err := s.existing(u, req.Path, ccfs.Modify)
	if err != nil {
		return err
	}
	if err := s.fs.Carry(r.Context(), u, from, ccfs.Modify); err != nil {
		return err
	}
	if path.Base(from.Path) == req.Name {
		return v1.WriteJSON(w, r, http.StatusOK, Changed{Path: from.Path})
	}
	to, err := s.newEntry(u, path.Join(path.Dir(from.Path), req.Name))
	if err == errExists && sameFile(from.Real, to.Real) {
		err = nil
	}
	if err != nil {
		return err
	}
	if err := renameNoReplace(from.Real, to.Real); err != nil {
		return err
	}
	fschange.Moved(from.Path, to.Path)
	s.moved(r.Context(), labels.Move{From: from.Path, To: to.Path})
	out := Changed{Path: to.Path}
	out.Undo = s.record(u, undo.Step{Kind: StepRename, Name: path.Base(from.Path), Count: 1, Path: path.Dir(from.Path), To: req.Name},
		renameData{From: from.Path, To: to.Path})
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// preflightSize is what a copy or move would carry. A copy leaves out what is
// hidden, and the answer has to say nothing about it.
func preflightSize(ctx context.Context, fsys *ccfs.Resolver, u *users.User, src ccfs.Location, move bool) (bytes, files int64, err error) {
	if move {
		return jobs.TreeSize(ctx, src.Real)
	}
	return jobs.VisibleSize(ctx, fsys, u, src, srcPermFor(move))
}

// renameNoReplace renames unless the target exists. A plain rename would
// silently replace a file of that name. On NTFS, exFAT and FAT the new name
// finds the file itself when only its case changes, and that rename goes.
func renameNoReplace(from, to string) error {
	if _, err := os.Lstat(to); err == nil && !sameFile(from, to) {
		return errExists
	}
	return os.Rename(from, to)
}

// sameFile reports a name that differs only in case and finds the file
// itself. A second hard link of the file has a name of its own, and a
// rename onto it would do nothing at all.
func sameFile(a, b string) bool {
	if !strings.EqualFold(filepath.Base(a), filepath.Base(b)) {
		return false
	}
	ia, err := os.Lstat(a)
	if err != nil {
		return false
	}
	ib, err := os.Lstat(b)
	return err == nil && os.SameFile(ia, ib)
}

// existing checks that p exists inside a source and u holds perm there.
func (s *Service) existing(u *users.User, p string, perm ccfs.Perm) (ccfs.Location, error) {
	loc, err := s.fs.Entry(u, p, perm)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" {
		return loc, v1.Coded(http.StatusBadRequest, "outsideSources", "only entries inside a source can be changed")
	}
	if _, err := os.Lstat(loc.Real); err != nil {
		return loc, err
	}
	return loc, nil
}

// newEntry checks that u may create p and that its folder exists.
func (s *Service) newEntry(u *users.User, p string) (ccfs.Location, error) {
	loc, err := s.fs.Entry(u, p, ccfs.Create)
	if err != nil {
		return loc, err
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" || !ccfs.ValidName(path.Base(loc.Path)) {
		return loc, v1.Coded(http.StatusBadRequest, "badPath", "invalid path")
	}
	if err := windowsName(loc); err != nil {
		return loc, err
	}
	parent, err := s.fs.Resolve(path.Dir(loc.Path))
	if err != nil {
		return loc, err
	}
	if err := folder(parent); err != nil {
		return loc, err
	}
	if _, err := os.Lstat(loc.Real); err == nil {
		return loc, errExists
	}
	return loc, nil
}

// windowsName turns away a name Windows refuses, before the system does it
// with a message that tells nobody why.
func windowsName(loc ccfs.Location) error {
	if !ccfs.WindowsNames(filepath.Dir(loc.Real)) {
		return nil
	}
	switch ccfs.WindowsNameProblem(path.Base(loc.Path)) {
	case "nameChars":
		return v1.Coded(http.StatusBadRequest, "nameChars", `Windows allows none of < > : " / \ | ? * in a name`)
	case "nameEnd":
		return v1.Coded(http.StatusBadRequest, "nameEnd", "Windows does not allow a name to end with a dot or a space")
	case "nameReserved":
		return v1.Coded(http.StatusBadRequest, "nameReserved", "Windows keeps this name for a device")
	}
	return nil
}

// record adds an undo step and returns its id, or "" when the step could
// not be kept. The operation itself went through, so that is no error.
func (s *Service) record(u *users.User, step undo.Step, data any) string {
	if s.undo == nil {
		return ""
	}
	st, err := s.undo.Record(u.Username, step, data)
	if err != nil {
		return ""
	}
	return st.ID
}

type preflightRequest struct {
	Kind string `json:"kind"`
	TransferParams
}

// Preflight is what a copy or move would run into.
type Preflight struct {
	Dest      string          `json:"dest"`
	Count     int             `json:"count"`
	Bytes     int64           `json:"bytes"`
	Files     int64           `json:"files"`
	Conflicts []jobs.Conflict `json:"conflicts"`
	// Same names the entries whose target is the same file under another
	// path. The job leaves them alone, so they are no conflict and carry
	// nothing.
	Same []string `json:"same"`
	// SameDevice says that a move only renames and needs no free space.
	SameDevice bool   `json:"sameDevice"`
	Space      *Space `json:"space,omitempty"`
	// Big copies and moves wait for a running parity check.
	Big        bool `json:"big"`
	ParityBusy bool `json:"parityBusy"`
	// Unraid adds the disk share trap, hard links the transfer breaks,
	// the space after the allocation rules of the share and the disks it
	// wakes. It is missing where Unraid has nothing to say.
	Unraid *unraid.TransferReport `json:"unraid,omitempty"`
}

func (s *Service) preflight(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req preflightRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Kind != KindCopy && req.Kind != KindMove {
		return v1.Errorf(http.StatusBadRequest, "kind must be copy or move")
	}
	move := req.Kind == KindMove
	dest, plan, err := check(r.Context(), s.fs, u, req.TransferParams, move)
	if err != nil {
		return err
	}
	out := Preflight{Dest: dest.Path, Count: len(plan.Items), Conflicts: []jobs.Conflict{}, Same: []string{}, SameDevice: move}
	destInfo, err := os.Stat(dest.Real)
	if err != nil {
		return err
	}
	destDev, devKnown := device(destInfo)
	for _, it := range plan.Items {
		src, err := s.fs.Resolve(it.Path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(src.Real)
		if err != nil {
			return err
		}
		if dev, ok := device(info); !ok || !devKnown || dev != destDev {
			out.SameDevice = false
		}
		name := it.Name
		if name == "" {
			name = path.Base(src.Path)
		}
		dst := node{dest.Path, dest.Real}.child(name)
		var target os.FileInfo
		if _, err := s.fs.Check(u, dst.api, ccfs.View); err == nil {
			target, _ = os.Lstat(dst.real)
		}
		switch {
		case target == nil:
		case os.SameFile(info, target) || unraidService().SameFile(src.Path, dst.api):
			out.Same = append(out.Same, name)
			continue
		default:
			out.Conflicts = append(out.Conflicts, jobs.NewConflict(name, side(node{src.Path, src.Real}, info), side(dst, target)))
		}
		b, f, err := preflightSize(r.Context(), s.fs, u, src, move)
		if err != nil {
			return err
		}
		out.Bytes += b
		out.Files += f
	}
	need := out.Bytes
	if out.SameDevice {
		need = 0
	}
	out.Space = plainSpace(dest, need)
	out.Big = out.Bytes >= jobs.BigJob
	if s.jobs != nil {
		out.ParityBusy = s.jobs.ParityBusy()
	}
	paths := make([]string, len(plan.Items))
	for i, it := range plan.Items {
		paths[i] = it.Path
	}
	report, err := s.unraid.CheckTransfer(r.Context(), unraid.Transfer{Kind: req.Kind, Paths: paths, Dest: dest.Path})
	report.Keep(func(p string) bool {
		_, err := s.fs.Check(u, p, ccfs.View)
		return err == nil
	})
	if err != nil {
		logger.Errorf("preflight: %v", err)
	} else if report.Trap != nil || report.Links != nil || report.Space != nil || len(report.Wakes) > 0 || report.Lands != "" || report.Linkable {
		out.Unraid = &report
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}
