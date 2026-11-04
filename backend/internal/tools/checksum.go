package tools

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"lukechampine.com/blake3"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
)

// Job kinds for checksums.
const (
	KindChecksum = "checksum"
	KindVerify   = "verify"
)

func init() {
	jobs.RegisterKind(KindChecksum, jobs.Kind{Prepare: prepareChecksum, Run: runChecksum})
	jobs.RegisterKind(KindVerify, jobs.Kind{Prepare: prepareVerify, Run: runVerify})
}

// Algo is a checksum algorithm, named like the ending of its sum files.
type Algo struct {
	ID    string
	Tag   string
	Hex   int
	Maker func() hash.Hash
}

// Algos are the algorithms in the order the prototype offers them.
var Algos = []Algo{
	{"md5", "MD5", 32, md5.New},
	{"sha1", "SHA1", 40, sha1.New},
	{"sha256", "SHA256", 64, sha256.New},
	{"sha512", "SHA512", 128, sha512.New},
	{"b3", "BLAKE3", 64, func() hash.Hash { return blake3.New(32, nil) }},
}

func algoByID(id string) (Algo, bool) {
	for _, a := range Algos {
		if a.ID == id {
			return a, true
		}
	}
	return Algo{}, false
}

// sumFile matches the lists this tool writes and reads, which are left out
// when a folder gets checksums.
var sumFile = regexp.MustCompile(`(?i)\.(md5|sha1|sha256|sha512|b3)$|^(SHA256SUMS|SHA1SUMS|MD5SUMS|SHA512SUMS|B3SUMS)$`)

// Sum hashes the content of r.
func Sum(a Algo, r io.Reader) (string, error) {
	h := a.Maker()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FormatLine writes one line of a sum file. GNU style is what sha256sum
// prints and reads with -c, BSD style what macOS and shasum --tag write.
// A name with a backslash or a newline gets the escaping coreutils uses.
func FormatLine(a Algo, sum, name string, bsd bool) string {
	if bsd {
		return fmt.Sprintf("%s (%s) = %s\n", a.Tag, name, sum)
	}
	if strings.ContainsAny(name, "\\\n\r") {
		r := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r")
		return "\\" + sum + "  " + r.Replace(name) + "\n"
	}
	return sum + "  " + name + "\n"
}

// SumLine is one line of a sum file.
type SumLine struct {
	Name string `json:"name"`
	Hash string `json:"hash,omitempty"`
	Algo string `json:"algo,omitempty"`
	// Bad holds a line that is neither GNU nor BSD style.
	Bad string `json:"bad,omitempty"`
}

var (
	gnuLine = regexp.MustCompile(`^(\\?)([0-9a-fA-F]{32,128})\s[ *](.+)$`)
	bsdLine = regexp.MustCompile(`^(MD5|SHA1|SHA256|SHA512|BLAKE3)\s*\((.+)\)\s*=\s*([0-9a-fA-F]+)$`)
)

// ParseSums reads both styles. fileName, the name of the list, settles the
// algorithm where the line does not, as in a .b3 file whose hashes are as
// long as SHA-256 ones.
func ParseSums(text, fileName string) []SumLine {
	var out []SumLine
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if m := bsdLine.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			for _, a := range Algos {
				if a.Tag == m[1] && len(m[3]) == a.Hex {
					out = append(out, SumLine{Name: m[2], Hash: strings.ToLower(m[3]), Algo: a.ID})
				}
			}
			continue
		}
		if m := gnuLine.FindStringSubmatch(l); m != nil {
			name := m[3]
			if m[1] == "\\" {
				name = unescape(name)
			}
			out = append(out, SumLine{Name: name, Hash: strings.ToLower(m[2]), Algo: algoFor(fileName, len(m[2]))})
			continue
		}
		out = append(out, SumLine{Bad: l})
	}
	return out
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func algoFor(fileName string, hexLen int) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(fileName), "."))
	if a, ok := algoByID(ext); ok && a.Hex == hexLen {
		return a.ID
	}
	if strings.HasPrefix(strings.ToUpper(path.Base(fileName)), "B3") && hexLen == 64 {
		return "b3"
	}
	for _, a := range Algos {
		if a.Hex == hexLen {
			return a.ID
		}
	}
	return ""
}

// ChecksumParams are the params of checksum. The sources lie in one
// folder, and names in the result are relative to it.
type ChecksumParams struct {
	Sources []string `json:"sources"`
	Algo    string   `json:"algo"`
	// Save is the sum file to write, if any. Names in it are relative to
	// its folder, as sha256sum -c expects.
	Save string `json:"save,omitempty"`
	BSD  bool   `json:"bsd,omitempty"`
}

type checksumStored struct {
	Algo     string `json:"algo"`
	BSD      bool   `json:"bsd"`
	Base     string `json:"base"`
	BasePath string `json:"basePath"`
	Save     string `json:"save,omitempty"`
}

// FileSum is the checksum of one file.
type FileSum struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Hash string `json:"hash"`
}

func prepareChecksum(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p ChecksumParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	if p.Algo == "" {
		p.Algo = "sha256"
	}
	if _, ok := algoByID(p.Algo); !ok {
		return nil, badRequest("algo must be md5, sha1, sha256, sha512 or b3")
	}
	if len(p.Sources) == 0 {
		return nil, badRequest("sources are required")
	}
	if sameDir(p.Sources) == "" {
		return nil, badRequest("the sources have to lie in one folder")
	}
	st := checksumStored{Algo: p.Algo, BSD: p.BSD}
	plan := &jobs.Plan{}
	for i, s := range p.Sources {
		loc, _, err := entry(d.FS, u, s, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		st.Base, st.BasePath = filepath.Dir(loc.Real), path.Dir(loc.Path)
		plan.Items = append(plan.Items, jobs.Item{Seq: i, Path: loc.Path, Name: path.Base(loc.Path)})
	}
	if p.Save != "" {
		perm := ccfs.Create
		if loc, err := d.FS.Resolve(p.Save); err == nil {
			if _, err := os.Lstat(loc.Real); err == nil {
				perm |= ccfs.Modify
			}
		}
		save, err := d.FS.Check(u, p.Save, perm)
		if err != nil {
			return nil, err
		}
		if _, err := folder(d.FS, u, path.Dir(save.Path), ccfs.Create); err != nil {
			return nil, err
		}
		if !ccfs.ValidName(path.Base(save.Path)) {
			return nil, badRequest("invalid name")
		}
		st.Save = save.Path
		plan.Dest = path.Dir(save.Path)
	}
	plan.Params = st
	return plan, nil
}

func runChecksum(ctx context.Context, run *jobs.Run) error {
	var st checksumStored
	if err := run.Params(&st); err != nil {
		return err
	}
	a, _ := algoByID(st.Algo)
	var sums []FileSum
	for _, it := range run.Items() {
		root := filepath.Join(st.Base, path.Base(it.Path))
		var files []string
		var size int64
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && bookkeeping(d.Name()) {
				return filepath.SkipDir
			}
			if !d.Type().IsRegular() || sumFile.MatchString(d.Name()) || bookkeeping(d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			files = append(files, p)
			size += info.Size()
			return nil
		})
		if err != nil {
			run.Finish(it.Seq, jobs.ItemFailed, err)
			continue
		}
		run.Measure(it.Seq, size, int64(len(files)))
		run.Begin(it.Seq)
		var ierr error
		for _, f := range files {
			rel, _ := filepath.Rel(st.Base, f)
			rel = filepath.ToSlash(rel)
			run.Current(rel)
			sum, err := hashFile(ctx, run, a, f)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				ierr = err
				continue
			}
			if err := run.Advance(ctx, 0, 1); err != nil {
				return err
			}
			sums = append(sums, FileSum{Path: path.Join(st.BasePath, rel), Name: rel, Hash: sum})
		}
		run.Finish(it.Seq, jobs.ItemDone, ierr)
	}
	result := map[string]any{"algo": st.Algo, "files": sums}
	if st.Save != "" {
		if err := writeSums(run, st, a, sums); err != nil {
			return err
		}
		result["saved"] = st.Save
	}
	run.SetResult(result)
	return nil
}

func hashFile(ctx context.Context, run *jobs.Run, a Algo, p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := a.Maker()
	if _, err := copyCounted(ctx, run, h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeSums writes the list beside its files. Names are relative to the
// list's folder where the files lie below it, and full paths otherwise.
func writeSums(run *jobs.Run, st checksumStored, a Algo, sums []FileSum) error {
	save, err := run.Deps().FS.Resolve(st.Save)
	if err != nil {
		return err
	}
	dir := path.Dir(save.Path)
	var b strings.Builder
	for _, s := range sums {
		name := s.Path
		if rest, ok := strings.CutPrefix(s.Path, strings.TrimSuffix(dir, "/")+"/"); ok {
			name = rest
		}
		b.WriteString(FormatLine(a, s.Hash, name, st.BSD))
	}
	part := filepath.Join(filepath.Dir(save.Real), "."+filepath.Base(save.Real)+partSuffix)
	if err := os.WriteFile(part, []byte(b.String()), 0o666); err != nil {
		return err
	}
	if err := os.Rename(part, save.Real); err != nil {
		os.Remove(part)
		return err
	}
	fschange.Created(save.Path)
	return nil
}

// VerifyParams are the params of verify: a sum file, or pasted lines with
// the folder their names are relative to.
type VerifyParams struct {
	File string `json:"file,omitempty"`
	Text string `json:"text,omitempty"`
	Dir  string `json:"dir,omitempty"`
}

type verifyStored struct {
	Lines []SumLine `json:"lines"`
	Real  []string  `json:"real"`
}

// Checked is the outcome for one line: ok, mismatch, missing or bad.
type Checked struct {
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`
	Algo  string `json:"algo,omitempty"`
	State string `json:"state"`
}

const maxSumFile = 16 << 20

func prepareVerify(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p VerifyParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	text, dir, listName := p.Text, p.Dir, ""
	if p.File != "" {
		loc, info, err := file(d.FS, u, p.File, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, err
		}
		if info.Size() > maxSumFile {
			return nil, badRequest("the sum file is too big")
		}
		b, err := os.ReadFile(loc.Real)
		if err != nil {
			return nil, err
		}
		text, dir, listName = string(b), path.Dir(loc.Path), path.Base(loc.Path)
	} else if _, err := folder(d.FS, u, dir, ccfs.View); err != nil {
		return nil, err
	}
	lines := ParseSums(text, listName)
	if len(lines) == 0 {
		return nil, badRequest("there are no checksums in it")
	}
	st := verifyStored{Lines: lines, Real: make([]string, len(lines))}
	plan := &jobs.Plan{Name: listName}
	for i, l := range lines {
		it := jobs.Item{Seq: i, Name: l.Name}
		if l.Bad == "" {
			p := l.Name
			if !strings.HasPrefix(p, "/") {
				p = path.Join(dir, p)
			}
			if loc, _, err := file(d.FS, u, p, ccfs.View|ccfs.Download); err == nil {
				it.Path, st.Real[i] = loc.Path, loc.Real
			}
		}
		if it.Name == "" {
			it.Name = l.Bad
		}
		plan.Items = append(plan.Items, it)
	}
	if plan.Name == "" {
		plan.Name = lines[0].Name
	}
	plan.Params = st
	return plan, nil
}

func runVerify(ctx context.Context, run *jobs.Run) error {
	var st verifyStored
	if err := run.Params(&st); err != nil {
		return err
	}
	for _, it := range pending(run) {
		if real := st.Real[it.Seq]; real != "" {
			if info, err := os.Stat(real); err == nil {
				run.Measure(it.Seq, info.Size(), 1)
			}
		}
	}
	checked := make([]Checked, len(st.Lines))
	for i, l := range st.Lines {
		checked[i] = Checked{Name: l.Name, Algo: l.Algo, State: "missing"}
		if l.Bad != "" {
			checked[i] = Checked{Name: l.Bad, State: "bad"}
		}
	}
	for _, it := range run.Items() {
		l, real := st.Lines[it.Seq], st.Real[it.Seq]
		checked[it.Seq].Path = it.Path
		if l.Bad != "" || real == "" {
			if it.State == jobs.ItemPending {
				run.Finish(it.Seq, jobs.ItemSkipped, nil)
			}
			continue
		}
		a, ok := algoByID(l.Algo)
		if !ok {
			checked[it.Seq].State = "bad"
			run.Finish(it.Seq, jobs.ItemSkipped, nil)
			continue
		}
		run.Begin(it.Seq)
		run.Current(it.Name)
		sum, err := hashFile(ctx, run, a, real)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil:
			run.Finish(it.Seq, jobs.ItemFailed, err)
			continue
		case sum == l.Hash:
			checked[it.Seq].State = "ok"
		default:
			checked[it.Seq].State = "mismatch"
		}
		run.Finish(it.Seq, jobs.ItemDone, nil)
	}
	run.SetResult(map[string]any{"lines": checked})
	return nil
}

type parseRequest struct {
	Path string `json:"path,omitempty"`
	Text string `json:"text,omitempty"`
}

func (h handlers) checksumParse(w http.ResponseWriter, r *http.Request) error {
	var req parseRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	text, name := req.Text, ""
	if req.Path != "" {
		loc, info, err := file(h.fs, v1.UserFrom(r), req.Path, ccfs.View|ccfs.Download)
		if err != nil {
			return err
		}
		if info.Size() > maxSumFile {
			return badRequest("the sum file is too big")
		}
		b, err := os.ReadFile(loc.Real)
		if err != nil {
			return err
		}
		text, name = string(b), path.Base(loc.Path)
	}
	lines := ParseSums(text, name)
	if lines == nil {
		lines = []SumLine{}
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"lines": lines})
}
