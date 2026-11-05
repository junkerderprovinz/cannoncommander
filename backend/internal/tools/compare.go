package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// KindSync carries out the steps of a sync plan.
const KindSync = "sync"

func init() {
	jobs.RegisterKind(KindSync, jobs.Kind{Prepare: prepareSync, Run: runSync, Heavy: true})
}

// States of a compared name.
const (
	Same   = "same"
	NewerL = "newerL"
	NewerR = "newerR"
	Differ = "differ"
	OnlyL  = "onlyL"
	OnlyR  = "onlyR"
)

// timeSlack is how far apart two times may be and still count as equal.
// FAT and some network file systems keep only even seconds.
const timeSlack = 2 * time.Second

// Side is one entry of a compared pair.
type Side struct {
	Dir   bool      `json:"dir"`
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
}

// Compared is one name found on either side.
type Compared struct {
	Rel   string `json:"rel"`
	Name  string `json:"name"`
	State string `json:"state"`
	Left  *Side  `json:"left,omitempty"`
	Right *Side  `json:"right,omitempty"`

	lReal, rReal string
}

// CompareRequest compares the folders Left and Right. Top compares only
// the names directly inside, with folders by their total size, as the
// compare dialog does. Without it, folders on both sides are gone into
// when Recursive is set and left out otherwise.
type CompareRequest struct {
	Left      string `json:"left"`
	Right     string `json:"right"`
	Content   bool   `json:"content,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
	Top       bool   `json:"top,omitempty"`
	Hidden    bool   `json:"hidden,omitempty"`
}

type comparer struct {
	ctx context.Context
	req CompareRequest
	out []Compared
	fs  *ccfs.Resolver
	u   *users.User
	// perm is what the user has to hold on an entry for it to show up, and
	// sides are the API paths of the two folders.
	perm  ccfs.Perm
	sides [2]string
}

func sideOf(real string, info fs.FileInfo) *Side {
	s := &Side{Dir: info.IsDir(), Size: info.Size(), MTime: info.ModTime()}
	if s.Dir {
		s.Size, _, _ = jobs.TreeSize(context.Background(), real)
	}
	return s
}

func (c *comparer) walk(l, r, rel string) error {
	names := map[string][2]fs.FileInfo{}
	for i, dir := range []string{l, r} {
		if dir == "" {
			continue
		}
		list, err := os.ReadDir(dir)
		if err != nil && !isNotExist(err) {
			return err
		}
		for _, d := range list {
			if bookkeeping(d.Name()) || !c.req.Hidden && strings.HasPrefix(d.Name(), ".") {
				continue
			}
			if _, err := c.fs.Check(c.u, path.Join(c.sides[i], rel, d.Name()), c.perm); err != nil {
				continue
			}
			// Links count as themselves. Following them would walk out of the
			// folder the rights were checked for.
			info, err := os.Lstat(filepath.Join(dir, d.Name()))
			if err != nil {
				continue
			}
			pair := names[d.Name()]
			pair[i] = info
			names[d.Name()] = pair
		}
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, naturalCompare)
	for _, name := range keys {
		if err := c.ctx.Err(); err != nil {
			return err
		}
		pair := names[name]
		p := name
		if rel != "" {
			p = rel + "/" + name
		}
		lReal, rReal := filepath.Join(l, name), filepath.Join(r, name)
		if !c.req.Top && pair[0] != nil && pair[1] != nil && pair[0].IsDir() && pair[1].IsDir() {
			if c.req.Recursive {
				if err := c.walk(lReal, rReal, p); err != nil {
					return err
				}
			}
			continue
		}
		it := Compared{Rel: p, Name: name, lReal: lReal, rReal: rReal}
		if pair[0] != nil {
			it.Left = sideOf(lReal, pair[0])
		}
		if pair[1] != nil {
			it.Right = sideOf(rReal, pair[1])
		}
		it.State = c.state(it)
		c.out = append(c.out, it)
	}
	return nil
}

func (c *comparer) state(it Compared) string {
	l, r := it.Left, it.Right
	switch {
	case r == nil:
		return OnlyL
	case l == nil:
		return OnlyR
	case l.Dir || r.Dir:
		if l.Dir && r.Dir && l.Size == r.Size {
			return Same
		}
		return Differ
	}
	dt := l.MTime.Sub(r.MTime)
	if c.req.Content {
		if l.Size == r.Size && sameContent(it.lReal, it.rReal) {
			return Same
		}
	} else if l.Size == r.Size && dt.Abs() < timeSlack {
		return Same
	}
	switch {
	case dt > timeSlack:
		return NewerL
	case dt < -timeSlack:
		return NewerR
	}
	return Differ
}

func sameContent(a, b string) bool {
	for _, p := range []string{a, b} {
		if info, err := os.Lstat(p); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	ha, err := fileSum(a)
	if err != nil {
		return false
	}
	hb, err := fileSum(b)
	return err == nil && bytes.Equal(ha, hb)
}

func fileSum(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// naturalCompare sorts names as people read them, so file2 comes before
// file10.
func naturalCompare(a, b string) int {
	al, bl := strings.ToLower(a), strings.ToLower(b)
	for al != "" && bl != "" {
		an, ar := leadingNumber(al)
		bn, br := leadingNumber(bl)
		if an != "" && bn != "" {
			an, bn = strings.TrimLeft(an, "0"), strings.TrimLeft(bn, "0")
			if len(an) != len(bn) {
				return len(an) - len(bn)
			}
			if c := strings.Compare(an, bn); c != 0 {
				return c
			}
			al, bl = ar, br
			continue
		}
		if al[0] != bl[0] {
			return int(al[0]) - int(bl[0])
		}
		al, bl = al[1:], bl[1:]
	}
	if c := len(al) - len(bl); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func leadingNumber(s string) (string, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}

func (h handlers) pair(r *http.Request, req CompareRequest) (ccfs.Location, ccfs.Location, error) {
	u := v1.UserFrom(r)
	perm := comparePerm(req)
	l, err := folder(h.fs, u, req.Left, perm)
	if err != nil {
		return l, l, err
	}
	rr, err := folder(h.fs, u, req.Right, perm)
	if err != nil {
		return l, rr, err
	}
	if l.Path == rr.Path {
		return l, rr, badRequest("both sides are the same folder")
	}
	return l, rr, nil
}

// comparePerm is what a comparison shows of each entry: its name and size, and
// with Content whether two files hold the same bytes.
func comparePerm(req CompareRequest) ccfs.Perm {
	if req.Content {
		return ccfs.View | ccfs.Download
	}
	return ccfs.View
}

func compareFolders(ctx context.Context, fsys *ccfs.Resolver, u *users.User, l, r ccfs.Location, req CompareRequest) ([]Compared, error) {
	c := &comparer{ctx: ctx, req: req, fs: fsys, u: u, perm: comparePerm(req), sides: [2]string{l.Path, r.Path}}
	err := c.walk(l.Real, r.Real, "")
	return c.out, err
}

func (h handlers) compare(w http.ResponseWriter, r *http.Request) error {
	var req CompareRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	l, rr, err := h.pair(r, req)
	if err != nil {
		return err
	}
	items, err := compareFolders(r.Context(), h.fs, v1.UserFrom(r), l, rr, req)
	if err != nil {
		return err
	}
	counts := map[string]int{Same: 0, NewerL: 0, NewerR: 0, Differ: 0, OnlyL: 0, OnlyR: 0}
	for _, it := range items {
		counts[it.State]++
	}
	if items == nil {
		items = []Compared{}
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"items": items, "counts": counts})
}

// Step is one change of a sync plan.
type Step struct {
	Key string `json:"key"`
	Rel string `json:"rel"`
	// Act is copy, replace or delete.
	Act string `json:"act"`
	// To is the side written to, L or R.
	To  string `json:"to"`
	Dir bool   `json:"dir,omitempty"`
	// Off marks a step that would overwrite something newer and starts
	// switched off. Why says what is risky about it: rightNewer,
	// leftNewer or sameAge.
	Off  bool   `json:"off,omitempty"`
	Why  string `json:"why,omitempty"`
	Size int64  `json:"size"`
}

// SyncPlanRequest asks for the steps that make two folders agree.
// Direction is right, left or both. Mirror deletes on the target what the
// source lacks and only goes with one direction.
type SyncPlanRequest struct {
	Left      string `json:"left"`
	Right     string `json:"right"`
	Direction string `json:"direction"`
	Content   bool   `json:"content,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
	Mirror    bool   `json:"mirror,omitempty"`
	Hidden    bool   `json:"hidden,omitempty"`
}

// Plan turns compared items into steps, as the prototype does.
func Plan(items []Compared, direction string, mirror bool) []Step {
	steps := []Step{}
	add := func(it Compared, act, to string, off bool, why string) {
		src := it.Left
		if act == "delete" && to == "R" || act != "delete" && to == "L" {
			src = it.Right
		}
		steps = append(steps, Step{Key: it.Rel + "|" + act + "|" + to, Rel: it.Rel, Act: act, To: to, Dir: src.Dir, Off: off, Why: why, Size: src.Size})
	}
	for _, it := range items {
		s := it.State
		if s == Same {
			continue
		}
		switch direction {
		case "right":
			switch s {
			case OnlyL:
				add(it, "copy", "R", false, "")
			case OnlyR:
				if mirror {
					add(it, "delete", "R", false, "")
				}
			default:
				add(it, "replace", "R", s != NewerL, map[string]string{NewerR: "rightNewer", Differ: "sameAge"}[s])
			}
		case "left":
			switch s {
			case OnlyR:
				add(it, "copy", "L", false, "")
			case OnlyL:
				if mirror {
					add(it, "delete", "L", false, "")
				}
			default:
				add(it, "replace", "L", s != NewerR, map[string]string{NewerL: "leftNewer", Differ: "sameAge"}[s])
			}
		default:
			switch s {
			case OnlyL:
				add(it, "copy", "R", false, "")
			case OnlyR:
				add(it, "copy", "L", false, "")
			case NewerL:
				add(it, "replace", "R", false, "")
			case NewerR:
				add(it, "replace", "L", false, "")
			default:
				add(it, "replace", "R", true, "sameAge")
			}
		}
	}
	return steps
}

func (h handlers) syncPlan(w http.ResponseWriter, r *http.Request) error {
	var req SyncPlanRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains([]string{"right", "left", "both"}, req.Direction) {
		return badRequest("direction must be right, left or both")
	}
	if req.Mirror && req.Direction == "both" {
		return badRequest("mirroring needs one direction")
	}
	cr := CompareRequest{Left: req.Left, Right: req.Right, Content: req.Content, Recursive: req.Recursive, Hidden: req.Hidden}
	l, rr, err := h.pair(r, cr)
	if err != nil {
		return err
	}
	items, err := compareFolders(r.Context(), h.fs, v1.UserFrom(r), l, rr, cr)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"steps": Plan(items, req.Direction, req.Mirror)})
}

// SyncParams are the params of sync: the steps that are switched on.
type SyncParams struct {
	Left  string     `json:"left"`
	Right string     `json:"right"`
	Steps []SyncStep `json:"steps"`
}

// SyncStep is a step of the plan as the job takes it.
type SyncStep struct {
	Rel string `json:"rel"`
	Act string `json:"act"`
	To  string `json:"to"`
}

func prepareSync(ctx context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p SyncParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if len(p.Steps) == 0 {
		return nil, badRequest("there is nothing to do")
	}
	need := map[string]ccfs.Perm{"L": 0, "R": 0}
	for _, s := range p.Steps {
		if s.To != "L" && s.To != "R" {
			return nil, badRequest("to must be L or R")
		}
		other := map[string]string{"L": "R", "R": "L"}[s.To]
		switch s.Act {
		case "copy":
			need[s.To] |= ccfs.Create
			need[other] |= ccfs.Download
		case "replace":
			need[s.To] |= ccfs.Create | ccfs.Modify
			need[other] |= ccfs.Download
		case "delete":
			need[s.To] |= ccfs.Delete
		default:
			return nil, badRequest("act must be copy, replace or delete")
		}
		if _, err := archives.CleanName(s.Rel); err != nil {
			return nil, badRequest("invalid step " + s.Rel)
		}
	}
	l, err := folder(d.FS, u, p.Left, ccfs.View|need["L"])
	if err != nil {
		return nil, err
	}
	r, err := folder(d.FS, u, p.Right, ccfs.View|need["R"])
	if err != nil {
		return nil, err
	}
	if l.Path == r.Path {
		return nil, badRequest("both sides are the same folder")
	}
	p.Left, p.Right = l.Path, r.Path
	plan := &jobs.Plan{Params: p, Name: path.Base(l.Path), Dest: r.Path}
	for i, s := range p.Steps {
		if err := p.mayDo(ctx, d.FS, u, s); err != nil {
			return nil, err
		}
		base := l.Path
		if s.To == "R" {
			base = r.Path
		}
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: path.Join(base, s.Rel), Name: s.Rel})
	}
	return plan, nil
}

// sides gives the API paths a step reads from and writes to.
func (p SyncParams) sides(s SyncStep) (src, dst string) {
	src, dst = path.Join(p.Left, s.Rel), path.Join(p.Right, s.Rel)
	if s.To == "L" {
		return dst, src
	}
	return src, dst
}

// writePerm is what a step takes at the place it writes to.
func (s SyncStep) writePerm() ccfs.Perm {
	switch s.Act {
	case "delete":
		return ccfs.Delete
	case "replace":
		return ccfs.Create | ccfs.Modify
	}
	return ccfs.Create
}

// mayDo checks one step for u. The two folders of a sync passing says nothing
// about the paths below them, which come from the client.
func (p SyncParams) mayDo(ctx context.Context, fsys *ccfs.Resolver, u *users.User, s SyncStep) error {
	src, dst := p.sides(s)
	if s.Act != "delete" {
		if _, err := fsys.Reach(u, src, syncReadPerm); err != nil {
			return err
		}
	}
	loc, err := fsys.Reach(u, dst, s.writePerm())
	if err != nil {
		return err
	}
	if s.Act == "copy" {
		return nil
	}
	return fsys.Whole(ctx, u, loc, s.writePerm())
}

const syncReadPerm = ccfs.View | ccfs.Download

func runSync(ctx context.Context, run *jobs.Run) error {
	var p SyncParams
	if err := run.Params(&p); err != nil {
		return err
	}
	fsys := run.Deps().FS
	l, err := fsys.Resolve(p.Left)
	if err != nil {
		return err
	}
	r, err := fsys.Resolve(p.Right)
	if err != nil {
		return err
	}
	for _, it := range pending(run) {
		s := p.Steps[it.Seq]
		rel := filepath.FromSlash(s.Rel)
		srcPath, dstPath := p.sides(s)
		src, dst := filepath.Join(l.Real, rel), filepath.Join(r.Real, rel)
		if s.To == "L" {
			src, dst = dst, src
		}
		run.Begin(it.Seq)
		run.Current(s.Rel)
		u, err := run.User()
		if err == nil {
			err = p.mayDo(ctx, fsys, u, s)
		}
		if err == nil {
			if s.Act == "delete" {
				err = syncDelete(ctx, run, dstPath)
			} else {
				if size, files, terr := jobs.TreeSize(ctx, src); terr == nil {
					run.Measure(it.Seq, size, files)
				}
				may := func(rel string) bool {
					if _, err := fsys.Check(u, path.Join(srcPath, rel), syncReadPerm); err != nil {
						return false
					}
					_, err := fsys.Check(u, path.Join(dstPath, rel), s.writePerm())
					return err == nil
				}
				err = copyTree(ctx, run, src, dst, "", may)
				if err == nil {
					fschange.Created(dstPath)
				}
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run.Finish(it.Seq, jobs.ItemDone, err)
	}
	return nil
}

func syncDelete(ctx context.Context, run *jobs.Run, p string) error {
	bin := trash.For(run.Deps())
	if bin == nil {
		return os.ErrInvalid
	}
	loc, err := run.Deps().FS.Resolve(p)
	if err != nil {
		return err
	}
	_, err = bin.Put(ctx, run.Owner(), loc)
	if isNotExist(err) {
		return nil
	}
	return err
}

// copyTree copies src to dst, replacing files of the same name and adding
// to folders that exist, with modes and times kept. Every file goes
// through a part file first. Below src it leaves out what may refuses.
func copyTree(ctx context.Context, run *jobs.Run, src, dst, rel string, may func(rel string) bool) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if old, err := os.Lstat(dst); err == nil && !old.IsDir() {
			os.Remove(dst)
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if old, err := os.Lstat(dst); err == nil && !old.IsDir() {
			if err := os.Remove(dst); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		list, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, d := range list {
			child := path.Join(rel, d.Name())
			if bookkeeping(d.Name()) || !may(child) {
				continue
			}
			if err := copyTree(ctx, run, filepath.Join(src, d.Name()), filepath.Join(dst, d.Name()), child, may); err != nil {
				return err
			}
		}
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	case !info.Mode().IsRegular():
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := writePart(ctx, run, dst, in); err != nil {
		return err
	}
	_ = os.Chmod(dst, info.Mode().Perm())
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
