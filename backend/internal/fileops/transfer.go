package fileops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// Job kinds of this package.
const (
	KindCopy   = "copy"
	KindMove   = "move"
	KindDelete = "delete"
)

// maxUndoEntries bounds what a copy or move remembers for taking it back.
// A job that touched more is not undoable.
const maxUndoEntries = 10000

var (
	errVerify  = errors.New("checksum mismatch after copying")
	errSpecial = errors.New("devices, sockets and pipes are not copied")
)

func init() {
	jobs.RegisterKind(KindCopy, jobs.Kind{Prepare: prepare(false), Run: runTransfer(false), Heavy: true})
	jobs.RegisterKind(KindMove, jobs.Kind{Prepare: prepare(true), Run: runTransfer(true), Heavy: true})
}

// TransferParams are the params of copy and move.
type TransferParams struct {
	Sources []string `json:"sources"`
	Dest    string   `json:"dest"`
	// Names renames the sources at the target, in the same order.
	Names []string `json:"names,omitempty"`
	// Rule answers every conflict without a decision of its own. ask, the
	// default, asks through job.conflict.
	Rule string `json:"rule,omitempty"`
	// Decisions answer conflicts up front, keyed like Conflict.Key, for
	// example from the preflight.
	Decisions map[string]string `json:"decisions,omitempty"`
	// Verify reads every copied file back and compares checksums.
	Verify bool `json:"verify,omitempty"`
	// KeepAttrs keeps owner, mode and times. It is on unless set to false.
	KeepAttrs *bool `json:"keepAttrs,omitempty"`
	// NewFolder makes Dest first. Taking the job back also removes it again,
	// so the two count as one step.
	NewFolder bool `json:"newFolder,omitempty"`
}

func prepare(move bool) func(context.Context, v1.Deps, *users.User, json.RawMessage) (*jobs.Plan, error) {
	return func(ctx context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
		var p TransferParams
		if err := jobs.Decode(raw, &p); err != nil {
			return nil, err
		}
		_, plan, err := check(ctx, d.FS, u, p, move)
		return plan, err
	}
}

// check validates a copy or move for u and plans it. It returns the
// destination folder as well, for the preflight.
func check(ctx context.Context, fsys *ccfs.Resolver, u *users.User, p TransferParams, move bool) (ccfs.Location, *jobs.Plan, error) {
	if len(p.Sources) == 0 {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "sources are required")
	}
	if len(p.Names) > 0 && len(p.Names) != len(p.Sources) {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "names must match the sources")
	}
	if p.Rule == "" {
		p.Rule = jobs.Ask
	}
	if !jobs.ValidRule(p.Rule) {
		return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "rule must be ask, skip, replace, both or newer")
	}
	for k, c := range p.Decisions {
		if !jobs.ValidChoice(c) || k == "" {
			return ccfs.Location{}, nil, v1.Errorf(http.StatusBadRequest, "decisions must be skip, replace, both or newer")
		}
	}
	if p.KeepAttrs == nil {
		keep := true
		p.KeepAttrs = &keep
	}
	destPerm := destPermFor(p.Rule)
	dest, err := fsys.Reach(u, p.Dest, destPerm)
	if err != nil {
		return dest, nil, err
	}
	if p.NewFolder {
		err = freshFolder(fsys, u, dest, move)
	} else {
		err = folder(dest)
	}
	if err != nil {
		return dest, nil, err
	}
	srcPerm := srcPermFor(move)
	plan := &jobs.Plan{Dest: dest.Path, Answers: p.Decisions, Rule: p.Rule}
	for i, raw := range p.Sources {
		src, err := fsys.Reach(u, raw, srcPerm)
		if err != nil {
			return dest, nil, err
		}
		if src.Virtual() || src.InArchive() || src.Index == "/" {
			return dest, nil, v1.Errorf(http.StatusBadRequest, "only files and folders inside a source can be copied or moved")
		}
		if _, err := os.Lstat(src.Real); err != nil {
			return dest, nil, err
		}
		// A copy leaves out what is hidden, but a move takes the folder along.
		if move {
			if err := fsys.Whole(ctx, u, src, srcPerm); err != nil {
				return dest, nil, err
			}
		}
		name := path.Base(src.Path)
		if len(p.Names) > 0 {
			name = p.Names[i]
		}
		if !ccfs.ValidName(name) {
			return dest, nil, v1.Errorf(http.StatusBadRequest, "invalid name")
		}
		if _, err := fsys.Check(u, path.Join(dest.Path, name), destPerm); err != nil {
			return dest, nil, err
		}
		if path.Join(dest.Path, name) == src.Path {
			return dest, nil, v1.Errorf(http.StatusBadRequest, "source and target are the same folder")
		}
		if dest.Path == src.Path || strings.HasPrefix(dest.Path, src.Path+"/") {
			return dest, nil, v1.Errorf(http.StatusBadRequest, "a folder cannot go into itself")
		}
		p.Sources[i] = src.Path
		item := jobs.Item{Path: src.Path}
		if name != path.Base(src.Path) {
			item.Name = name
		}
		plan.Items = append(plan.Items, item)
	}
	plan.Params = p
	return dest, plan, nil
}

// freshFolder checks that a folder can be made at loc.
func freshFolder(fsys *ccfs.Resolver, u *users.User, loc ccfs.Location, move bool) error {
	if !move {
		return v1.Errorf(http.StatusBadRequest, "only a move can make its folder")
	}
	if loc.Virtual() || loc.InArchive() || loc.Index == "/" || !ccfs.ValidName(path.Base(loc.Path)) {
		return v1.Errorf(http.StatusBadRequest, "invalid path")
	}
	parent, err := fsys.Reach(u, path.Dir(loc.Path), ccfs.Create)
	if err != nil {
		return err
	}
	if err := folder(parent); err != nil {
		return err
	}
	if _, err := os.Lstat(loc.Real); err == nil {
		return errExists
	}
	return nil
}

func srcPermFor(move bool) ccfs.Perm {
	if move {
		return ccfs.View | ccfs.Delete
	}
	return ccfs.View | ccfs.Download
}

func destPermFor(rule string) ccfs.Perm {
	if rule != jobs.Skip && rule != jobs.Both {
		return ccfs.Create | ccfs.Modify
	}
	return ccfs.Create
}

// folder makes sure loc is a real folder that can take entries.
func folder(loc ccfs.Location) error {
	if loc.Virtual() || loc.InArchive() {
		return v1.Errorf(http.StatusBadRequest, "the target has to be a folder inside a source")
	}
	info, err := os.Stat(loc.Real)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return v1.Errorf(http.StatusBadRequest, "the target is not a folder")
	}
	return nil
}

// node is one entry by its API path and its path on disk.
type node struct{ api, real string }

func (n node) child(name string) node {
	return node{path.Join(n.api, name), filepath.Join(n.real, name)}
}

func (n node) sibling(name string) node {
	return node{path.Join(path.Dir(n.api), name), filepath.Join(filepath.Dir(n.real), name)}
}

type replacedEntry struct {
	Path    string `json:"path"`
	TrashID string `json:"trashId"`
}

type movedEntry struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// transferUndo is what taking back a copy or move needs.
type transferUndo struct {
	Created  []string        `json:"created,omitempty"`
	Moved    []movedEntry    `json:"moved,omitempty"`
	Replaced []replacedEntry `json:"replaced,omitempty"`
	// Folder was made for the job and goes away with it.
	Folder string `json:"folder,omitempty"`
}

type transfer struct {
	s    *Service
	run  *jobs.Run
	move bool
	keep bool
	hash bool
	// resumed is set while an item runs that was cut off before, so
	// files this job already wrote are not taken for conflicts.
	resumed bool
	buf     []byte
	undo    transferUndo
	tooBig  bool
	// follow are the labels to carry along once the item is through.
	follow []labels.Move

	// u is the owner as of the last look, and the two rights are what the job
	// needs at its sources and at its target.
	u                *users.User
	srcPerm, dstPerm ccfs.Perm
}

func runTransfer(move bool) func(context.Context, *jobs.Run) error {
	return func(ctx context.Context, run *jobs.Run) error {
		s := For(run.Deps())
		if s == nil {
			return errors.New("no file service")
		}
		var p TransferParams
		if err := run.Params(&p); err != nil {
			return err
		}
		dest, err := s.fs.Resolve(p.Dest)
		if err != nil {
			return err
		}
		t := &transfer{s: s, run: run, move: move, keep: p.KeepAttrs == nil || *p.KeepAttrs, hash: p.Verify,
			srcPerm: srcPermFor(move), dstPerm: destPermFor(p.Rule)}
		if p.NewFolder {
			// A restart finds the folder there already.
			if _, _, err := run.Reach(p.Dest, ccfs.Create); err != nil {
				return err
			}
			if err := os.Mkdir(dest.Real, 0o777); err == nil {
				fschange.Created(dest.Path)
			} else if !errors.Is(err, fs.ErrExist) {
				return err
			}
			t.undo.Folder = dest.Path
		}

		items := run.Items()
		for _, it := range items {
			if it.State != jobs.ItemPending {
				continue
			}
			if loc, err := s.fs.Resolve(it.Path); err == nil {
				if b, f, err := jobs.TreeSize(ctx, loc.Real); err == nil {
					run.Measure(it.Seq, b, f)
				}
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		for _, it := range items {
			if it.State != jobs.ItemPending {
				continue
			}
			run.Begin(it.Seq)
			skipped, err := t.item(ctx, dest, it)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			state := jobs.ItemDone
			if skipped {
				state = jobs.ItemSkipped
			}
			if move {
				s.moved(ctx, t.follow...)
			} else {
				s.copied(ctx, t.follow...)
			}
			t.follow = t.follow[:0]
			run.Finish(it.Seq, state, err)
		}
		t.record(p, run.Items())
		return nil
	}
}

func (t *transfer) item(ctx context.Context, dest ccfs.Location, it jobs.Item) (bool, error) {
	src, u, err := t.run.Reach(it.Path, t.srcPerm)
	if err != nil {
		return false, err
	}
	t.u = u
	if err := t.s.fs.Follows(u, dest, t.dstPerm); err != nil {
		return false, err
	}
	if err := folder(dest); err != nil {
		return false, err
	}
	if t.move {
		if err := t.s.fs.Whole(ctx, u, src, t.srcPerm); err != nil {
			return false, err
		}
	}
	name := it.Name
	if name == "" {
		name = path.Base(src.Path)
	}
	t.resumed = it.Resumed
	from := node{src.Path, src.Real}
	to := node{dest.Path, dest.Real}.child(name)
	return t.entry(ctx, from, to, name, false)
}

// entry copies or moves src to dst. key is dst relative to the target
// folder. inNew is set inside a folder this job created, where nothing can
// clash and nothing needs to be remembered for undo on its own. It reports
// whether src was skipped as a whole.
func (t *transfer) entry(ctx context.Context, src, dst node, key string, inNew bool) (bool, error) {
	t.run.Current(key)
	info, err := os.Lstat(src.real)
	if err != nil {
		return false, err
	}
	target, err := os.Lstat(dst.real)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	replace, merge := false, false
	if exists {
		switch {
		case t.resumed && ours(info, target):
			if info.IsDir() {
				merge = true
				break
			}
			// Written before the restart, only the source is left over.
			if t.move {
				if err := os.Remove(src.real); err != nil {
					return false, err
				}
			}
			t.remember(src, dst, inNew)
			return false, t.run.Advance(ctx, sizeOf(info), 1)
		default:
			choice, err := t.choose(ctx, key, src, dst, info, target)
			if err != nil {
				return false, err
			}
			// An answer can take a long time to come.
			if t.u, err = t.run.User(); err != nil {
				return false, err
			}
			if _, err := t.s.fs.Check(t.u, src.api, t.srcPerm); err != nil {
				return false, err
			}
			bothDirs := info.IsDir() && target.IsDir()
			switch choice {
			case jobs.Skip:
				return true, nil
			case jobs.Both:
				dst = dst.sibling(ccfs.FreeName(filepath.Dir(dst.real), filepath.Base(dst.real)))
				exists = false
			case jobs.Newer:
				if !bothDirs && !newer(info, target) {
					return true, nil
				}
				fallthrough
			case jobs.Replace:
				if bothDirs {
					merge = true
				} else {
					replace = true
				}
			}
		}
	}

	if err := t.mayWrite(ctx, dst, replace); err != nil {
		return false, err
	}

	if t.move && !merge {
		if replace {
			if err := t.discard(ctx, dst); err != nil {
				return false, err
			}
			replace = false
		}
		err := os.Rename(src.real, dst.real)
		if err == nil {
			fschange.Moved(src.api, dst.api)
			t.remember(src, dst, inNew)
			return false, nil
		}
		if !ccfs.CrossDevice(err) {
			return false, err
		}
		exists = false
	}

	switch {
	case info.IsDir():
		return false, t.dir(ctx, src, dst, key, info, exists && merge, replace, inNew)
	case info.Mode()&fs.ModeSymlink != 0:
		return false, t.link(ctx, src, dst, info, replace, inNew)
	case info.Mode().IsRegular():
		return false, t.file(ctx, src, dst, info, replace, inNew)
	}
	return false, errSpecial
}

// mayWrite checks the owner's rights at dst. Only the job's top entries were
// checked when it started, and a rule can name any place below the target.
func (t *transfer) mayWrite(ctx context.Context, dst node, replace bool) error {
	loc, err := t.s.fs.Check(t.u, dst.api, t.dstPerm)
	if err != nil || !replace {
		return err
	}
	return t.s.fs.Whole(ctx, t.u, loc, t.dstPerm)
}

// choose finds the answer for a conflict, from the decisions, the rule or
// the user.
func (t *transfer) choose(ctx context.Context, key string, src, dst node, info, target fs.FileInfo) (string, error) {
	if c := t.run.Answer(key); c != "" {
		return c, nil
	}
	c := jobs.NewConflict(key, side(src, info), side(dst, target))
	return t.run.Ask(ctx, c)
}

func side(n node, info fs.FileInfo) jobs.Side {
	s := jobs.Side{Path: n.api, Dir: info.IsDir(), Size: sizeOf(info), MTime: info.ModTime()}
	if s.Dir {
		if entries, err := os.ReadDir(n.real); err == nil {
			s.Entries = len(entries)
		}
	}
	return s
}

func (t *transfer) dir(ctx context.Context, src, dst node, key string, info fs.FileInfo, merge, replace, inNew bool) error {
	created := false
	if !merge {
		if replace {
			if err := t.discard(ctx, dst); err != nil {
				return err
			}
		}
		if err := os.Mkdir(dst.real, 0o777); err != nil {
			return err
		}
		created = true
		t.remember(src, dst, inNew)
		fschange.Created(dst.api)
	}
	entries, err := os.ReadDir(src.real)
	if err != nil {
		return err
	}
	var first error
	for _, e := range entries {
		name := e.Name()
		if IsPart(name) {
			continue
		}
		if _, err := t.s.fs.Check(t.u, src.child(name).api, t.srcPerm); err != nil {
			continue
		}
		if _, err := t.entry(ctx, src.child(name), dst.child(name), key+"/"+name, inNew || created); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			first = cmpOr(first, err)
		}
	}
	if created && t.keep {
		_ = os.Chmod(dst.real, info.Mode().Perm())
		Chown(dst.real, info)
		_ = os.Chtimes(dst.real, info.ModTime(), info.ModTime())
	}
	if t.move && first == nil {
		// Skipped entries keep the folder alive, and that is fine.
		_ = os.Remove(src.real)
	}
	if err := t.run.Advance(ctx, 0, 1); err != nil {
		return err
	}
	return first
}

func (t *transfer) link(ctx context.Context, src, dst node, info fs.FileInfo, replace, inNew bool) error {
	target, err := os.Readlink(src.real)
	if err != nil {
		return err
	}
	if replace {
		if err := t.discard(ctx, dst); err != nil {
			return err
		}
	}
	if err := os.Symlink(target, dst.real); err != nil {
		return err
	}
	if t.keep {
		Chown(dst.real, info)
	}
	t.remember(src, dst, inNew)
	fschange.Created(dst.api)
	if t.move {
		if err := os.Remove(src.real); err != nil {
			return err
		}
	}
	return t.run.Advance(ctx, 0, 1)
}

func (t *transfer) file(ctx context.Context, src, dst node, info fs.FileInfo, replace, inNew bool) (err error) {
	part := filepath.Join(filepath.Dir(dst.real), "."+filepath.Base(dst.real)+PartSuffix)
	defer func() {
		// A shutdown keeps the part file, the next start goes on from it.
		if err != nil && (ctx.Err() == nil || t.run.Cancelled()) {
			_ = os.Remove(part)
		}
	}()
	if err := t.copyData(ctx, src.real, part, info); err != nil {
		return err
	}
	if t.hash {
		if err := t.verify(ctx, src.real, part); err != nil {
			return err
		}
	}
	if t.keep {
		_ = os.Chmod(part, info.Mode().Perm())
		Chown(part, info)
		if err := os.Chtimes(part, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	if replace {
		if err := t.discard(ctx, dst); err != nil {
			return err
		}
	}
	if err := os.Rename(part, dst.real); err != nil {
		return err
	}
	t.remember(src, dst, inNew)
	fschange.Created(dst.api)
	if t.move {
		if err := os.Remove(src.real); err != nil {
			return err
		}
	}
	return t.run.Advance(ctx, 0, 1)
}

// copyData writes src into part in chunks, each one a pause point. A part
// file left by an earlier run of this item is continued.
func (t *transfer) copyData(ctx context.Context, src, part string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	var offset int64
	if t.resumed {
		if pi, err := os.Stat(part); err == nil && pi.Size() <= info.Size() {
			offset = pi.Size()
		}
	}
	flags := os.O_WRONLY | os.O_CREATE
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	out, err := ccfs.OpenPart(part, flags)
	if err != nil {
		return err
	}
	defer out.Close()
	if offset > 0 {
		if _, err := in.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		if _, err := out.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		if err := t.run.Advance(ctx, offset, 0); err != nil {
			return err
		}
	}
	if t.buf == nil {
		t.buf = make([]byte, jobs.Chunk)
	}
	for {
		n, rerr := in.Read(t.buf)
		if n > 0 {
			if err := t.run.Transfer(ctx, n); err != nil {
				return err
			}
			if _, err := out.Write(t.buf[:n]); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return out.Close()
}

// verify reads both files again and compares their checksums. It waits at
// every chunk, so a pause also holds the check.
func (t *transfer) verify(ctx context.Context, src, part string) error {
	a, err := t.sum(ctx, src)
	if err != nil {
		return err
	}
	b, err := t.sum(ctx, part)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return errVerify
	}
	return nil
}

func (t *transfer) sum(ctx context.Context, p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var h hash.Hash = sha256.New()
	for {
		n, err := f.Read(t.buf)
		h.Write(t.buf[:n])
		if err == io.EOF {
			return h.Sum(nil), nil
		}
		if err != nil {
			return nil, err
		}
		if err := t.run.Wait(ctx); err != nil {
			return nil, err
		}
	}
}

// discard makes room for a replacement. The old entry goes to the trash
// when it can, so taking the job back brings it back. Where there is no
// trash, it is deleted.
func (t *transfer) discard(ctx context.Context, dst node) error {
	if t.s.trash != nil {
		if loc, err := t.s.fs.Resolve(dst.api); err == nil {
			if it, err := t.s.trash.Put(ctx, t.run.Owner(), loc); err == nil {
				t.undo.Replaced = append(t.undo.Replaced, replacedEntry{Path: dst.api, TrashID: it.ID})
				return nil
			}
		}
	}
	if err := os.RemoveAll(dst.real); err != nil {
		return err
	}
	fschange.Removed(dst.api)
	t.s.removed(ctx, dst.api)
	return nil
}

// remember notes what taking the job back has to undo. Entries inside a
// folder the job created go with that folder.
func (t *transfer) remember(src, dst node, inNew bool) {
	if inNew {
		return
	}
	t.follow = append(t.follow, labels.Move{From: src.api, To: dst.api})
	if t.tooBig {
		return
	}
	if t.move {
		t.undo.Moved = append(t.undo.Moved, movedEntry{From: src.api, To: dst.api})
	} else {
		t.undo.Created = append(t.undo.Created, dst.api)
	}
	if len(t.undo.Created)+len(t.undo.Moved) > maxUndoEntries {
		t.tooBig = true
	}
}

func (t *transfer) record(p TransferParams, items []jobs.Item) {
	l := t.s.undo
	if l == nil || t.tooBig || len(t.undo.Created)+len(t.undo.Moved) == 0 {
		return
	}
	var first jobs.Item
	n := 0
	for _, it := range items {
		if it.State == jobs.ItemDone {
			if n == 0 {
				first = it
			}
			n++
		}
	}
	if n == 0 {
		return
	}
	kind := KindCopy
	if t.move {
		kind = KindMove
	}
	step := undo.Step{Kind: kind, Name: path.Base(first.Path), Count: n, Path: path.Dir(first.Path), To: p.Dest}
	if step, err := l.Record(t.run.Owner(), step, t.undo); err == nil {
		t.run.SetUndo(step.ID)
	}
}

// ours tells whether target is what an earlier run of this item wrote:
// a folder, a link to the same place, or a file of the same size and time.
func ours(info, target fs.FileInfo) bool {
	switch {
	case info.IsDir() || target.IsDir():
		return info.IsDir() && target.IsDir()
	case info.Mode()&fs.ModeSymlink != 0:
		return target.Mode()&fs.ModeSymlink != 0
	}
	return target.Mode().IsRegular() && info.Size() == target.Size() && !newer(info, target) && !newer(target, info)
}

// newer reports whether a changed more than a second after b.
func newer(a, b fs.FileInfo) bool { return a.ModTime().Sub(b.ModTime()) > time.Second }

func sizeOf(info fs.FileInfo) int64 {
	if info.Mode().IsRegular() {
		return info.Size()
	}
	return 0
}

func cmpOr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

// IsPart reports whether name is a part file of this package.
func IsPart(name string) bool {
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, PartSuffix)
}
