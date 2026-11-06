package unraid

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// FixPermsKind is the job kind that runs a PermsTask.
const FixPermsKind = "fixPerms"

// Owner and modes as Unraid's New Permissions tool sets them: nobody:users,
// so the shares, the containers and the network all may read and write.
const (
	permUID  = 99
	permGID  = 100
	dirMode  = 0o777
	fileMode = 0o666
)

// scanBudget caps how long a scan walks before it answers with what it has.
const scanBudget = 10 * time.Second

// Reasons a folder keeps its owners.
const (
	KeepAppdata = "appdata"
	KeepSystem  = "system"
	KeepDomains = "domains"
	KeepFlash   = "flash"
)

// keptShares maps the shares that keep their owners to the reason.
// Containers and VMs run with their own users in there, resetting them
// breaks databases and running guests.
func (s *Service) keptShares() map[string]string {
	kept := map[string]string{"appdata": KeepAppdata, "system": KeepSystem, "domains": KeepDomains}
	add := func(file, key, reason string) {
		secs, err := readSections(filepath.Join(s.cfg.BootConfig, file))
		if err != nil || len(secs) == 0 {
			return
		}
		parts := strings.Split(strings.Trim(secs[0].str(key), "/"), "/")
		if len(parts) >= 3 && parts[0] == "mnt" {
			kept[parts[2]] = reason
		}
	}
	add("docker.cfg", "DOCKER_APP_CONFIG_PATH", KeepAppdata)
	add("docker.cfg", "DOCKER_IMAGE_FILE", KeepSystem)
	add("domain.cfg", "DOMAINDIR", KeepDomains)
	add("domain.cfg", "IMAGE_FILE", KeepSystem)
	return kept
}

// keptReason returns why apiPath keeps its owners, or "".
func keptReason(apiPath string, kept map[string]string) string {
	if apiPath == "/boot" || strings.HasPrefix(apiPath, "/boot/") {
		return KeepFlash
	}
	parts := strings.Split(strings.Trim(apiPath, "/"), "/")
	if len(parts) >= 3 && parts[0] == "mnt" {
		return kept[parts[2]]
	}
	return ""
}

// PermsScan is what setting the rights in a folder would touch.
type PermsScan struct {
	Path string `json:"path"`
	// Blocked is the reason the folder itself is left alone.
	Blocked string `json:"blocked,omitempty"`
	Owner   string `json:"owner"`
	Files   int    `json:"files"`
	Dirs    int    `json:"dirs"`
	// Wrong counts entries whose owner or mode would change.
	Wrong   int        `json:"wrong"`
	Kept    []KeptPath `json:"kept"`
	Partial bool       `json:"partial,omitempty"`
}

// KeptPath is a folder the run leaves as it is.
type KeptPath struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// Size is the share's usage when the folder is a whole user share.
	Size int64 `json:"size,omitempty"`
}

// ScanPerms counts what a fixPerms run in loc would change.
func (s *Service) ScanPerms(ctx context.Context, loc ccfs.Location) (PermsScan, error) {
	out := PermsScan{Path: loc.Path, Owner: strconv.Itoa(permUID) + ":" + strconv.Itoa(permGID), Kept: []KeptPath{}}
	kept := s.keptShares()
	switch {
	case loc.InArchive():
		out.Blocked = "archive"
		return out, nil
	case keptReason(loc.Path, kept) != "":
		out.Blocked = keptReason(loc.Path, kept)
		return out, nil
	}
	st, err := s.State()
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, scanBudget)
	defer cancel()
	err = filepath.WalkDir(loc.Real, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil || out.Files+out.Dirs >= maxSurvey {
			out.Partial = true
			return filepath.SkipAll
		}
		apiPath := apiPathOf(loc, p)
		if why := keptReason(apiPath, kept); why != "" {
			if !d.IsDir() {
				return nil
			}
			k := KeptPath{Path: apiPath, Reason: why}
			if pl, ok := placeOf(st, apiPath); ok && pl.User() && pl.Rel == pl.Share {
				sh, _ := st.Share(pl.Share)
				k.Size = sh.Used
			}
			out.Kept = append(out.Kept, k)
			return filepath.SkipDir
		}
		if p != loc.Real {
			if d.IsDir() {
				out.Dirs++
			} else {
				out.Files++
			}
		}
		if info, err := d.Info(); err == nil && needsFix(info) {
			out.Wrong++
		}
		return nil
	})
	return out, err
}

func apiPathOf(loc ccfs.Location, real string) string {
	rel, err := filepath.Rel(loc.Real, real)
	if err != nil || rel == "." {
		return loc.Path
	}
	return path.Join(loc.Path, filepath.ToSlash(rel))
}

func wantMode(info fs.FileInfo) fs.FileMode {
	if info.IsDir() {
		return dirMode
	}
	return fileMode
}

func needsFix(info fs.FileInfo) bool {
	uid, gid, ok := ownerOf(info)
	if ok && (uid != permUID || gid != permGID) {
		return true
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	return info.Mode()&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != wantMode(info)
}

// PermsTask is a fixPerms run that passed its checks.
type PermsTask struct {
	Path string `json:"path"`

	real string
	kept map[string]string
}

// Step is called before each entry with its API path. It may block while
// the job is paused, and an error stops the run.
type Step func(apiPath string) error

// PermsResult is what a run did.
type PermsResult struct {
	Entries    int        `json:"entries"`
	Changed    int        `json:"changed"`
	Failed     int        `json:"failed"`
	FirstError string     `json:"firstError,omitempty"`
	Kept       []KeptPath `json:"kept"`
}

// PreparePerms checks that u may reset the rights below apiPath.
func (s *Service) PreparePerms(r *ccfs.Resolver, u *users.User, apiPath string) (PermsTask, error) {
	if u == nil || !u.Permissions.Admin {
		return PermsTask{}, v1.Coded(http.StatusForbidden, "adminsOnly", "admins only")
	}
	loc, err := r.Check(u, apiPath, ccfs.Modify)
	if err != nil {
		return PermsTask{}, err
	}
	return s.permsTask(loc)
}

// ResumePerms rebuilds a task that PreparePerms passed before, for a job
// that goes on after a restart. The rights of the user were checked then,
// the rest is checked again.
func (s *Service) ResumePerms(r *ccfs.Resolver, apiPath string) (PermsTask, error) {
	loc, err := r.Resolve(apiPath)
	if err != nil {
		return PermsTask{}, err
	}
	if loc.Virtual() {
		return PermsTask{}, v1.Coded(http.StatusBadRequest, "notAFolder", "path is not a folder")
	}
	return s.permsTask(loc)
}

func (s *Service) permsTask(loc ccfs.Location) (PermsTask, error) {
	kept := s.keptShares()
	switch {
	case loc.InArchive():
		return PermsTask{}, v1.Coded(http.StatusBadRequest, "inArchive", "path is inside an archive")
	case keptReason(loc.Path, kept) != "":
		return PermsTask{}, v1.Errorf(http.StatusConflict, "path keeps its owners: "+keptReason(loc.Path, kept))
	case !strings.HasPrefix(loc.Path, "/mnt/"):
		return PermsTask{}, v1.Coded(http.StatusBadRequest, "rightsBelowMnt", "rights are only set below /mnt")
	}
	if err := ccfs.Contained(loc); err != nil {
		return PermsTask{}, err
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return PermsTask{}, err
	}
	if !info.IsDir() {
		return PermsTask{}, v1.Coded(http.StatusBadRequest, "notAFolder", "path is not a folder")
	}
	return PermsTask{Path: loc.Path, real: loc.Real, kept: kept}, nil
}

// Run sets nobody:users with 0777 on folders and 0666 on files, skipping
// the folders that keep their owners. Links get the owner only and are
// never followed, so the run stays inside the folder.
func (t PermsTask) Run(ctx context.Context, step Step) (PermsResult, error) {
	res := PermsResult{Kept: []KeptPath{}}
	loc := ccfs.Location{Path: t.Path, Real: t.real}
	err := filepath.WalkDir(t.real, func(p string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		apiPath := apiPathOf(loc, p)
		if err != nil {
			res.fail(err)
			return nil
		}
		if why := keptReason(apiPath, t.kept); why != "" {
			if !d.IsDir() {
				return nil
			}
			res.Kept = append(res.Kept, KeptPath{Path: apiPath, Reason: why})
			return filepath.SkipDir
		}
		if step != nil {
			if err := step(apiPath); err != nil {
				return err
			}
		}
		res.Entries++
		info, err := d.Info()
		if err != nil {
			res.fail(err)
			return nil
		}
		if !needsFix(info) {
			return nil
		}
		if err := fix(p, info); err != nil {
			res.fail(err)
			return nil
		}
		res.Changed++
		return nil
	})
	return res, err
}

func (r *PermsResult) fail(err error) {
	r.Failed++
	if r.FirstError == "" {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		r.FirstError = err.Error()
	}
}

func fix(p string, info fs.FileInfo) error {
	if err := os.Lchown(p, permUID, permGID); err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil
	}
	return os.Chmod(p, wantMode(info))
}
