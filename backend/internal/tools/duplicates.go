package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// KindDuplicates is the job that finds files with the same content.
const KindDuplicates = "duplicates"

func init() {
	jobs.RegisterKind(KindDuplicates, jobs.Kind{Prepare: prepareDuplicates, Run: runDuplicates, Heavy: true, BigResult: true})
}

const (
	// headTail is how much of each end of a file the quick comparison
	// reads. Files that differ mostly differ there already, in a header
	// or a trailer.
	headTail = 64 << 10
	// maxGroups bounds the result a job keeps.
	maxGroups = 5000
)

// DuplicatesParams are the choices of the window. By is "name" for files
// with the same name and size, "size" for the same size alone, which also
// finds renamed copies. Either way the content decides in the end.
type DuplicatesParams struct {
	Path    string `json:"path"`
	By      string `json:"by,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
	MinSize int64  `json:"minSize,omitempty"`
}

// DupCopy is one file of a group.
type DupCopy struct {
	Path  string `json:"path"`
	At    string `json:"at"`
	MTime string `json:"mtime"`
	mtime time.Time
}

// DupGroup is a set of files with the same content. The copies come newest
// first, on equal times the one with the shorter path first, which the
// window treats as the original.
type DupGroup struct {
	Name   string    `json:"name"`
	Size   int64     `json:"size"`
	SHA256 string    `json:"sha256"`
	Wasted int64     `json:"wasted"`
	Copies []DupCopy `json:"copies"`
}

// Duplicates is the result of the job.
type Duplicates struct {
	Groups []DupGroup `json:"groups"`
	// More tells that there were more groups than the result keeps.
	More   bool  `json:"more,omitempty"`
	Wasted int64 `json:"wasted"`
	Files  int64 `json:"files"`
}

func prepareDuplicates(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p DuplicatesParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	switch p.By {
	case "":
		p.By = "name"
	case "name", "size":
	default:
		return nil, v1.Errorf(http.StatusBadRequest, "by is name or size")
	}
	if p.MinSize < 0 {
		return nil, v1.Errorf(http.StatusBadRequest, "minSize must not be negative")
	}
	loc, err := scanRoot(d.FS, u, p.Path)
	if err != nil {
		return nil, err
	}
	p.Path = loc.Path
	return &jobs.Plan{Params: p, Name: path.Base(loc.Path), Items: []jobs.Item{{Path: loc.Path, Name: path.Base(loc.Path)}}}, nil
}

func (h scanTools) duplicates(w http.ResponseWriter, r *http.Request) error {
	m := jobs.For(h.deps)
	if m == nil {
		return v1.Errorf(http.StatusServiceUnavailable, "the job queue is not running")
	}
	var p DuplicatesParams
	if err := v1.DecodeJSON(r, &p); err != nil {
		return err
	}
	raw, _ := json.Marshal(p)
	u := v1.UserFrom(r)
	plan, err := prepareDuplicates(r.Context(), h.deps, u, raw)
	if err != nil {
		return err
	}
	j, err := m.Submit(u.Username, KindDuplicates, plan, 0, false)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusAccepted, j)
}

type dupCandidate struct {
	fsindex.Entry
	key string
}

func runDuplicates(ctx context.Context, run *jobs.Run) error {
	var p DuplicatesParams
	if err := run.Params(&p); err != nil {
		return err
	}
	it := run.Items()[0]
	if it.State != jobs.ItemPending {
		return nil
	}
	u, err := run.User()
	if err != nil {
		return fmt.Errorf("the user who started this search is gone: %w", err)
	}
	fsys := run.Deps().FS
	loc, err := scanRoot(fsys, u, p.Path)
	if err != nil {
		run.Finish(it.Seq, jobs.ItemFailed, err)
		return nil
	}
	run.Begin(it.Seq)
	minSize := max(p.MinSize, 1)
	keyOf := func(e fsindex.Entry) string {
		if p.By == "size" {
			return fmt.Sprint(e.Size)
		}
		return fmt.Sprintf("%d|%s", e.Size, e.Name)
	}
	keepFile := func(e fsindex.Entry) bool { return !e.Dir && e.Size >= minSize }

	// The first walk only counts the keys, as small hashes, so a share of
	// millions of files does not have to be held in memory.
	seen := map[uint64]uint32{}
	var files int64
	walk := func(fn func(fsindex.Entry)) error {
		_, err := fsindex.Walk(ctx, loc, func(e fsindex.Entry) error {
			if !scanVisible(fsys, u, e, p.Hidden) {
				return fs.SkipDir
			}
			if keepFile(e) {
				fn(e)
			}
			return run.Wait(ctx)
		})
		return err
	}
	if err := walk(func(e fsindex.Entry) {
		files++
		seen[hashKey(keyOf(e))]++
	}); err != nil {
		return err
	}
	var cands []dupCandidate
	if err := walk(func(e fsindex.Entry) {
		if seen[hashKey(keyOf(e))] > 1 {
			cands = append(cands, dupCandidate{e, keyOf(e)})
		}
	}); err != nil {
		return err
	}
	seen = nil

	groups := regroupDups(cands, func(c *dupCandidate) (string, bool) { return c.key, true })
	var total int64
	for _, g := range groups {
		for _, c := range g {
			total += min(c.Size, 2*headTail)
		}
	}
	run.Measure(it.Seq, total, int64(len(cands)))
	groups = narrowDups(groups, func(c *dupCandidate) (string, bool) {
		sum, err := partialHash(ctx, run, c)
		return sum, err == nil && c.Size >= minSize
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	total = 0
	var n int64
	for _, g := range groups {
		for _, c := range g {
			total += c.Size
			n++
		}
	}
	run.Measure(it.Seq, total, n)
	sums := map[string]string{}
	groups = narrowDups(groups, func(c *dupCandidate) (string, bool) {
		sum, err := fullHash(ctx, run, *c)
		sums[c.Path] = sum
		return sum, err == nil
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	res := duplicateResult(groups, sums, files)
	run.SetResult(res)
	run.Finish(it.Seq, jobs.ItemDone, nil)
	return nil
}

func hashKey(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// regroupDups sorts candidates by key and keeps the groups of two or more.
func regroupDups(cands []dupCandidate, key func(*dupCandidate) (string, bool)) [][]dupCandidate {
	by := map[string][]dupCandidate{}
	var order []string
	for i := range cands {
		k, ok := key(&cands[i])
		if !ok {
			continue
		}
		if _, seen := by[k]; !seen {
			order = append(order, k)
		}
		by[k] = append(by[k], cands[i])
	}
	var out [][]dupCandidate
	for _, k := range order {
		if len(by[k]) > 1 {
			out = append(out, by[k])
		}
	}
	return out
}

// narrowDups splits every group by a finer key, a file that cannot be read
// drops out of its group.
func narrowDups(groups [][]dupCandidate, key func(*dupCandidate) (string, bool)) [][]dupCandidate {
	var out [][]dupCandidate
	for _, g := range groups {
		out = append(out, regroupDups(g, key)...)
	}
	return out
}

// partialHash also puts the length of the file into c, because the index
// may only know the blocks the file takes on its disk.
func partialHash(ctx context.Context, run *jobs.Run, c *dupCandidate) (string, error) {
	if ccfs.InCloud(c.Real) {
		return "", ccfs.ErrInCloud
	}
	f, err := ccfs.OpenRegular(c.Real)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	c.Size = info.Size()
	h := sha256.New()
	head := min(c.Size, headTail)
	if _, err := io.CopyN(h, f, head); err != nil {
		return "", err
	}
	if c.Size > 2*headTail {
		if _, err := f.Seek(-headTail, io.SeekEnd); err != nil {
			return "", err
		}
	}
	rest := min(c.Size-head, headTail)
	if _, err := io.CopyN(h, f, rest); err != nil {
		return "", err
	}
	if err := run.Transfer(ctx, int(head+rest)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fullHash reads the whole file through the job's speed limit, so a scan
// of a big share can be paused and slowed like a copy.
func fullHash(ctx context.Context, run *jobs.Run, c dupCandidate) (string, error) {
	if ccfs.InCloud(c.Real) {
		return "", ccfs.ErrInCloud
	}
	f, err := ccfs.OpenRegular(c.Real)
	if err != nil {
		return "", err
	}
	defer f.Close()
	run.Current(c.Name)
	h := sha256.New()
	buf := make([]byte, jobs.Chunk)
	for {
		n, err := f.Read(buf)
		h.Write(buf[:n])
		if n > 0 {
			if terr := run.Transfer(ctx, n); terr != nil {
				return "", terr
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func duplicateResult(groups [][]dupCandidate, sums map[string]string, files int64) Duplicates {
	out := Duplicates{Groups: []DupGroup{}, Files: files}
	for _, g := range groups {
		copies := make([]DupCopy, len(g))
		for i, c := range g {
			copies[i] = DupCopy{Path: c.Path, At: path.Dir(c.Path), MTime: stamp(c.MTime), mtime: c.MTime}
		}
		slices.SortStableFunc(copies, func(a, b DupCopy) int {
			if !a.mtime.Equal(b.mtime) {
				return b.mtime.Compare(a.mtime)
			}
			if d := len(a.At) - len(b.At); d != 0 {
				return d
			}
			return strings.Compare(a.Path, b.Path)
		})
		size := g[0].Size
		grp := DupGroup{Name: g[0].Name, Size: size, SHA256: sums[g[0].Path], Wasted: size * int64(len(g)-1), Copies: copies}
		out.Wasted += grp.Wasted
		out.Groups = append(out.Groups, grp)
	}
	slices.SortStableFunc(out.Groups, func(a, b DupGroup) int {
		if a.Wasted != b.Wasted {
			if a.Wasted > b.Wasted {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(out.Groups) > maxGroups {
		out.Groups, out.More = out.Groups[:maxGroups], true
	}
	return out
}
