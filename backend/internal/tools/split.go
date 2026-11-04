package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
)

// Job kinds for splitting a file and putting it back together.
const (
	KindSplit   = "split"
	KindCombine = "combine"
)

func init() {
	jobs.RegisterKind(KindSplit, jobs.Kind{Prepare: prepareSplit, Run: runSplit, Heavy: true})
	jobs.RegisterKind(KindCombine, jobs.Kind{Prepare: prepareCombine, Run: runCombine, Heavy: true})
}

// SplitParams are the params of split.
type SplitParams struct {
	Path string `json:"path"`
	Dest string `json:"dest"`
	// PartSize is the size of every part but the last, in bytes.
	PartSize int64 `json:"partSize"`
	// Sum lays a .sha256 of the whole file beside the parts, which combine
	// checks the result against.
	Sum bool `json:"sum,omitempty"`
}

// PartNames returns the names of the parts of a file of size bytes, as
// name.001 and on, with as many digits as the last number needs.
func PartNames(name string, size, part int64) []string {
	count := (size + part - 1) / part
	if size == 0 {
		count = 1
	}
	digits := max(3, len(strconv.FormatInt(count, 10)))
	out := make([]string, count)
	for i := range out {
		out[i] = fmt.Sprintf("%s.%0*d", name, digits, i+1)
	}
	return out
}

func prepareSplit(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p SplitParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	loc, info, err := file(d.FS, u, p.Path, ccfs.View|ccfs.Download)
	if err != nil {
		return nil, err
	}
	if p.PartSize < 1<<20 {
		return nil, badRequest("a part has to be at least 1 MiB")
	}
	if info.Size() <= p.PartSize {
		return nil, badRequest("the file is smaller than one part")
	}
	dest, err := folder(d.FS, u, p.Dest, ccfs.Create|ccfs.Modify)
	if err != nil {
		return nil, err
	}
	p.Path, p.Dest = loc.Path, dest.Path
	return &jobs.Plan{Params: p, Dest: dest.Path,
		Items: []jobs.Item{{Seq: 0, Path: loc.Path, Name: path.Base(loc.Path), Bytes: info.Size(), Files: 1}}}, nil
}

func runSplit(ctx context.Context, run *jobs.Run) error {
	var p SplitParams
	if err := run.Params(&p); err != nil {
		return err
	}
	it := run.Items()[0]
	if it.State == jobs.ItemDone {
		return nil
	}
	src, err := run.Deps().FS.Resolve(p.Path)
	if err != nil {
		return err
	}
	dest, err := run.Deps().FS.Resolve(p.Dest)
	if err != nil {
		return err
	}
	run.Begin(it.Seq)
	err = split(ctx, run, src.Real, dest.Real, p)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	run.Finish(it.Seq, jobs.ItemDone, err)
	if err != nil {
		return err
	}
	fschange.Created(p.Dest)
	return nil
}

func split(ctx context.Context, run *jobs.Run, src, dest string, p SplitParams) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	name := filepath.Base(src)
	sum := sha256.New()
	in := io.TeeReader(f, sum)
	names := PartNames(name, info.Size(), p.PartSize)
	for _, n := range names {
		run.Current(n)
		if err := writePart(ctx, run, filepath.Join(dest, n), io.LimitReader(in, p.PartSize)); err != nil {
			return err
		}
	}
	run.SetResult(map[string]any{"parts": len(names), "first": path.Join(p.Dest, names[0])})
	if !p.Sum {
		return nil
	}
	line := FormatLine(Algos[2], hex.EncodeToString(sum.Sum(nil)), name, false)
	return os.WriteFile(filepath.Join(dest, name+".sha256"), []byte(line), 0o666)
}

// writePart writes r to p through a part file, so a half written part
// never looks complete. An existing part of the same name is replaced.
func writePart(ctx context.Context, run *jobs.Run, p string, r io.Reader) error {
	part := filepath.Join(filepath.Dir(p), "."+filepath.Base(p)+partSuffix)
	out, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	_, err = copyCounted(ctx, run, out, r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(part, p)
	}
	if err != nil {
		os.Remove(part)
	}
	return err
}

// CombineParams are the params of combine. Path is any of the parts.
type CombineParams struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Dest string `json:"dest"`
	// Check compares the result with the .sha256 beside the parts.
	Check bool `json:"check,omitempty"`
	// Drop moves the parts to the trash once the result is complete and,
	// with Check, correct.
	Drop bool `json:"drop,omitempty"`
}

type combineStored struct {
	CombineParams
	Parts []string `json:"parts"`
	Sum   string   `json:"sum,omitempty"`
}

var partNumber = regexp.MustCompile(`^(.+)\.(\d{3,})$`)

// Parts finds the parts that belong with the part p in its folder, in
// order, and the numbers missing between them.
func Parts(p string) (base string, parts []string, missing []int, err error) {
	m := partNumber.FindStringSubmatch(filepath.Base(p))
	if m == nil {
		return "", nil, nil, errors.New("this is not a part such as file.001")
	}
	base = m[1]
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		return "", nil, nil, err
	}
	nums := map[int]string{}
	top := 0
	for _, e := range entries {
		if mm := partNumber.FindStringSubmatch(e.Name()); mm != nil && mm[1] == base && e.Type().IsRegular() {
			n, _ := strconv.Atoi(mm[2])
			nums[n] = e.Name()
			top = max(top, n)
		}
	}
	for i := 1; i <= top; i++ {
		if name, ok := nums[i]; ok {
			parts = append(parts, name)
		} else {
			missing = append(missing, i)
		}
	}
	return base, parts, missing, nil
}

func prepareCombine(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p CombineParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	perm := ccfs.View | ccfs.Download
	if p.Drop {
		perm |= ccfs.Delete
	}
	loc, _, err := file(d.FS, u, p.Path, perm)
	if err != nil {
		return nil, err
	}
	base, parts, missing, err := Parts(loc.Real)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	if len(missing) > 0 {
		return nil, badRequest(fmt.Sprintf("part %03d is missing", missing[0]))
	}
	if len(parts) < 2 {
		return nil, badRequest("there is only one part")
	}
	if p.Name == "" {
		p.Name = base
	}
	if !ccfs.ValidName(p.Name) {
		return nil, badRequest("invalid name")
	}
	dest, err := folder(d.FS, u, p.Dest, ccfs.Create)
	if err != nil {
		return nil, err
	}
	st := combineStored{CombineParams: p, Parts: parts}
	st.Path, st.Dest = path.Dir(loc.Path), dest.Path
	if p.Check {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(loc.Real), base+".sha256"))
		if err != nil {
			return nil, badRequest("there is no " + base + ".sha256 beside the parts to check against")
		}
		lines := ParseSums(string(b), base+".sha256")
		if len(lines) == 0 || lines[0].Bad != "" || lines[0].Algo != "sha256" {
			return nil, badRequest(base + ".sha256 holds no SHA-256 checksum")
		}
		st.Sum = lines[0].Hash
	}
	plan := &jobs.Plan{Params: st, Name: p.Name, Dest: dest.Path}
	for i, name := range parts {
		info, err := os.Stat(filepath.Join(filepath.Dir(loc.Real), name))
		if err != nil {
			return nil, err
		}
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: path.Join(st.Path, name), Name: name, Bytes: info.Size(), Files: 1})
	}
	return plan, nil
}

var errSum = errors.New("the checksum of the result does not match, the parts stay where they are")

func runCombine(ctx context.Context, run *jobs.Run) error {
	var st combineStored
	if err := run.Params(&st); err != nil {
		return err
	}
	fsys := run.Deps().FS
	dir, err := fsys.Resolve(st.Path)
	if err != nil {
		return err
	}
	dest, err := fsys.Resolve(st.Dest)
	if err != nil {
		return err
	}
	items := run.Items()
	if !slices.ContainsFunc(items, func(it jobs.Item) bool { return it.State != jobs.ItemDone }) {
		return nil
	}
	// A restart begins again with the first part, since the half written
	// result says nothing about where it stopped.
	name := ccfs.FreeName(dest.Real, st.Name)
	part := filepath.Join(dest.Real, "."+name+partSuffix)
	out, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	sum := sha256.New()
	w := io.MultiWriter(out, sum)
	for _, it := range items {
		run.Begin(it.Seq)
		run.Current(it.Name)
		err := appendFile(ctx, run, w, filepath.Join(dir.Real, it.Name))
		if err != nil {
			out.Close()
			os.Remove(part)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			run.Finish(it.Seq, jobs.ItemFailed, err)
			return err
		}
		run.Finish(it.Seq, jobs.ItemDone, nil)
	}
	if err := out.Close(); err != nil {
		os.Remove(part)
		return err
	}
	if st.Sum != "" && hex.EncodeToString(sum.Sum(nil)) != st.Sum {
		os.Remove(part)
		return errSum
	}
	name = ccfs.FreeName(dest.Real, name)
	if err := os.Rename(part, filepath.Join(dest.Real, name)); err != nil {
		os.Remove(part)
		return err
	}
	result := path.Join(st.Dest, name)
	fschange.Created(result)
	run.SetResult(map[string]any{"path": result, "checked": st.Sum != ""})
	if st.Drop {
		dropParts(ctx, run, items)
	}
	return nil
}

func appendFile(ctx context.Context, run *jobs.Run, w io.Writer, p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = copyCounted(ctx, run, w, f)
	return err
}

// dropParts moves the parts to the trash. The result is complete at this
// point, so a part that cannot go is only noted on its item.
func dropParts(ctx context.Context, run *jobs.Run, items []jobs.Item) {
	bin := trash.For(run.Deps())
	if bin == nil {
		return
	}
	for _, it := range items {
		loc, err := run.Deps().FS.Resolve(it.Path)
		if err == nil {
			_, err = bin.Put(ctx, run.Owner(), loc)
		}
		if err != nil && !isNotExist(err) {
			run.Finish(it.Seq, jobs.ItemDone, err)
		}
	}
}
