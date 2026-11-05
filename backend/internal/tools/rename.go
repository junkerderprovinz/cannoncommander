package tools

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// StepRenameBatch is the undo step of a batch rename.
const StepRenameBatch = "renameBatch"

func init() {
	undo.Register(StepRenameBatch, undo.Reverser{Inline: undoRenameBatch})
}

// Mask is a rename pattern as the prototype's dialog builds it. Name and
// Ext take the tokens [N] name, [E] ending, [D] date, [P] folder, [C]
// counter with [C3] for three digits, and from the metadata [A] taken,
// [K] camera, [I] artist, [T] title, [R] resolution, [S2] season and [F2]
// episode.
type Mask struct {
	Name  string `json:"name"`
	Ext   string `json:"ext"`
	Start int    `json:"start"`
	Step  int    `json:"step"`
	Find  string `json:"find,omitempty"`
	Repl  string `json:"repl,omitempty"`
	// Regex reads Find as a regular expression, with $1 and $2 for the
	// groups in Repl. Without it Find is plain text and ignores case.
	Regex bool `json:"regex,omitempty"`
	// Case is keep, lower, upper or title.
	Case string `json:"case,omitempty"`
}

var tokenRe = regexp.MustCompile(`\[([NEDPAKIRT])\]|\[([CSF])(\d?)\]`)

var metaToken = regexp.MustCompile(`\[[AKIRT]\]|\[[SF]\d?\]`)

// RenameRow is one entry of a batch rename.
type RenameRow struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
	// Error is empty, invalid, exists or duplicate.
	Error string `json:"error,omitempty"`
	// Missing names the metadata tokens the file had no value for.
	Missing []string `json:"missing,omitempty"`
}

type renameEntry struct {
	loc  ccfs.Location
	name string
	dir  bool
	info os.FileInfo
}

func splitExt(name string, dir bool) (string, string) {
	if dot := strings.LastIndex(name, "."); !dir && dot > 0 {
		return name[:dot], name[dot+1:]
	}
	return name, ""
}

func (m Mask) fill(mask string, e renameEntry, i int, meta func() map[byte]string, missing map[string]bool) string {
	base, ext := splitExt(e.name, e.dir)
	parent := path.Base(path.Dir(e.loc.Path))
	return tokenRe.ReplaceAllStringFunc(mask, func(tok string) string {
		g := tokenRe.FindStringSubmatch(tok)
		t, counted, pad := g[1], g[2], g[3]
		width, _ := strconv.Atoi(pad)
		switch t {
		case "N":
			return base
		case "E":
			return ext
		case "D":
			return e.info.ModTime().UTC().Format("2006-01-02")
		case "P":
			return parent
		}
		if counted == "C" {
			return padLeft(strconv.Itoa(m.Start+i*m.Step), width)
		}
		id := t + counted
		v := meta()[id[0]]
		if v == "" {
			missing[id] = true
		}
		if counted != "" {
			return padLeft(v, width)
		}
		return v
	})
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}

var caseBreak = regexp.MustCompile(`(^|[\s_\-.(])(\pL)`)

func applyCase(s, c string) string {
	switch c {
	case "lower":
		return strings.ToLower(s)
	case "upper":
		return strings.ToUpper(s)
	case "title":
		return caseBreak.ReplaceAllStringFunc(strings.ToLower(s), func(m string) string {
			r := []rune(m)
			r[len(r)-1] = unicode.ToUpper(r[len(r)-1])
			return string(r)
		})
	}
	return s
}

var jsGroup = regexp.MustCompile(`\$(\$|&|\d+)`)

// goReplacement turns the replacement syntax of JavaScript, which the
// prototype used, into Go's: $1x is group one and an x there, and $& is
// the whole match.
func goReplacement(repl string) string {
	return jsGroup.ReplaceAllStringFunc(repl, func(m string) string {
		switch m[1:] {
		case "$":
			return "$$"
		case "&":
			return "${0}"
		}
		return "${" + m[1:] + "}"
	})
}

func (m Mask) finder() (*regexp.Regexp, error) {
	if m.Find == "" {
		return nil, nil
	}
	if m.Regex {
		return regexp.Compile(m.Find)
	}
	return regexp.Compile("(?i)" + regexp.QuoteMeta(m.Find))
}

// apply returns the new name of every entry, in order, and the metadata
// tokens each one had no value for.
func (m Mask) apply(entries []renameEntry, meta Meta) ([]string, [][]string, error) {
	if m.Step == 0 {
		m.Step = 1
	}
	re, err := m.finder()
	if err != nil {
		return nil, nil, err
	}
	repl := goReplacement(m.Repl)
	needsMeta := metaToken.MatchString(m.Name + m.Ext)
	names := make([]string, len(entries))
	missing := make([][]string, len(entries))
	for i, e := range entries {
		var cached map[byte]string
		lookup := func() map[byte]string {
			if cached == nil {
				cached = map[byte]string{}
				if needsMeta && meta != nil {
					cached = meta(e.loc.Real, e.name)
				}
			}
			return cached
		}
		miss := map[string]bool{}
		out := strings.TrimSpace(m.fill(m.Name, e, i, lookup, miss))
		if !e.dir {
			if ext := m.fill(m.Ext, e, i, lookup, miss); ext != "" {
				out += "." + ext
			}
		}
		if re != nil {
			out = re.ReplaceAllString(out, repl)
		}
		names[i] = strings.TrimSpace(applyCase(out, m.Case))
		for id := range miss {
			missing[i] = append(missing[i], id)
		}
	}
	return names, missing, nil
}

// check fills in the error of every row: a name that is empty or not
// allowed, one taken by an entry that stays, or one given twice. Names
// are compared without case, as SMB clients on Windows and macOS see them.
func check(rows []RenameRow, entries []renameEntry) {
	moving := map[string]bool{}
	for _, e := range entries {
		moving[e.loc.Real] = true
	}
	taken := map[string]map[string]bool{}
	seen := map[string]int{}
	for i, r := range rows {
		dir := filepath.Dir(entries[i].loc.Real)
		if taken[dir] == nil {
			taken[dir] = map[string]bool{}
			if list, err := os.ReadDir(dir); err == nil {
				for _, d := range list {
					if !moving[filepath.Join(dir, d.Name())] {
						taken[dir][strings.ToLower(d.Name())] = true
					}
				}
			}
		}
		seen[dir+"\x00"+strings.ToLower(r.New)]++
	}
	for i := range rows {
		r := &rows[i]
		dir := filepath.Dir(entries[i].loc.Real)
		switch {
		case r.New == "":
			r.Error = "empty"
		case !ccfs.ValidName(r.New):
			r.Error = "invalid"
		case taken[dir][strings.ToLower(r.New)]:
			r.Error = "exists"
		case seen[dir+"\x00"+strings.ToLower(r.New)] > 1:
			r.Error = "duplicate"
		}
	}
}

// BatchRequest renames many entries, by a mask or with the new names
// given in order, as the editor does it.
type BatchRequest struct {
	Paths  []string `json:"paths"`
	Mask   *Mask    `json:"mask,omitempty"`
	Names  []string `json:"names,omitempty"`
	DryRun bool     `json:"dryRun,omitempty"`
}

// BatchResult is the answer of POST /fs/rename/batch.
type BatchResult struct {
	Rows    []RenameRow `json:"rows"`
	Changed int         `json:"changed"`
	Errors  int         `json:"errors"`
	Done    bool        `json:"done"`
	Undo    string      `json:"undo,omitempty"`
}

type renamePair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (h handlers) renameBatch(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	var req BatchRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.Paths) == 0 {
		return badRequest("paths are required")
	}
	if (req.Mask == nil) == (req.Names == nil) {
		return badRequest("give either a mask or the names")
	}
	if req.Names != nil && len(req.Names) != len(req.Paths) {
		return badRequest("there have to be as many names as paths")
	}
	entries := make([]renameEntry, len(req.Paths))
	for i, p := range req.Paths {
		loc, info, err := entry(h.fs, u, p, ccfs.Modify)
		if err != nil {
			return err
		}
		if _, err := folder(h.fs, u, path.Dir(loc.Path), ccfs.Create); err != nil {
			return err
		}
		if err := h.fs.Carry(r.Context(), u, loc, ccfs.Modify); err != nil {
			return err
		}
		entries[i] = renameEntry{loc: loc, name: path.Base(loc.Path), dir: info.IsDir(), info: info}
	}
	names, missing := req.Names, make([][]string, len(entries))
	if req.Mask != nil {
		var err error
		if names, missing, err = req.Mask.apply(entries, h.meta); err != nil {
			return badRequest("the expression is not valid")
		}
	}
	out := BatchResult{Rows: make([]RenameRow, len(entries))}
	for i, e := range entries {
		out.Rows[i] = RenameRow{Path: e.loc.Path, Old: e.name, New: strings.TrimSpace(names[i]), Missing: missing[i]}
	}
	check(out.Rows, entries)
	// A name the user may not use reads the same whether it is taken or not.
	for i := range out.Rows {
		row := &out.Rows[i]
		if row.New == row.Old || !ccfs.ValidName(row.New) {
			continue
		}
		if _, err := h.fs.Reach(u, path.Join(path.Dir(row.Path), row.New), ccfs.Create); err != nil {
			row.Error = "invalid"
		}
	}
	var todo []int
	for i, row := range out.Rows {
		if row.Error != "" {
			out.Errors++
		} else if row.New != row.Old {
			todo = append(todo, i)
		}
	}
	out.Changed = len(todo)
	if !req.DryRun && out.Errors > 0 {
		return v1.WriteJSON(w, r, http.StatusConflict, out)
	}
	if req.DryRun || len(todo) == 0 {
		return v1.WriteJSON(w, r, http.StatusOK, out)
	}
	pairs := make([]renamePair, 0, len(todo))
	for _, i := range todo {
		e := entries[i]
		pairs = append(pairs, renamePair{From: e.loc.Path, To: path.Join(path.Dir(e.loc.Path), out.Rows[i].New)})
	}
	if err := renamePairs(r.Context(), h.deps, pairs); err != nil {
		return err
	}
	out.Done = true
	if l := undo.For(h.deps); l != nil {
		first := entries[todo[0]]
		step := undo.Step{Kind: StepRenameBatch, Name: first.name, Count: len(pairs), Path: path.Dir(first.loc.Path)}
		if s, err := l.Record(u.Username, step, pairs); err == nil {
			out.Undo = s.ID
		}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// renamePairs renames in two rounds, first to a temporary name and then to
// the new one, so names can trade places and a change of case works on a
// file system that ignores case. A failure puts back what was renamed.
func renamePairs(ctx context.Context, d v1.Deps, pairs []renamePair) error {
	type step struct{ from, tmp, to string }
	steps := make([]step, 0, len(pairs))
	tag := strings.ToLower(rand.Text()[:8])
	for i, p := range pairs {
		from, err := d.FS.Resolve(p.From)
		if err != nil {
			return err
		}
		to, err := d.FS.Resolve(p.To)
		if err != nil {
			return err
		}
		if err := ccfs.Contained(to); err != nil {
			return err
		}
		tmp := filepath.Join(filepath.Dir(from.Real), ".cc-rename-"+tag+"-"+strconv.Itoa(i))
		steps = append(steps, step{from.Real, tmp, to.Real})
	}
	rollback := func(placed int) {
		for _, s := range steps[:placed] {
			_ = os.Rename(s.to, s.tmp)
		}
		for _, s := range steps {
			if _, err := os.Lstat(s.tmp); err == nil {
				_ = os.Rename(s.tmp, s.from)
			}
		}
	}
	for _, s := range steps {
		if err := os.Rename(s.from, s.tmp); err != nil {
			rollback(0)
			return err
		}
	}
	for i, s := range steps {
		if _, err := os.Lstat(s.to); err == nil {
			rollback(i)
			return v1.Errorf(http.StatusConflict, path.Base(pairs[i].To)+" exists already")
		}
		if err := os.Rename(s.tmp, s.to); err != nil {
			rollback(i)
			return err
		}
	}
	moves := make([]labels.Move, len(pairs))
	for i, p := range pairs {
		fschange.Moved(p.From, p.To)
		moves[i] = labels.Move{From: p.From, To: p.To}
	}
	if err := labels.Moved(ctx, d, moves...); err != nil {
		logger.Errorf("labels: %v", err)
	}
	return nil
}

func undoRenameBatch(ctx context.Context, d v1.Deps, u *users.User, raw json.RawMessage) error {
	var pairs []renamePair
	if err := json.Unmarshal(raw, &pairs); err != nil {
		return err
	}
	back := make([]renamePair, len(pairs))
	for i, p := range pairs {
		loc, _, err := entry(d.FS, u, p.To, ccfs.Modify)
		if err != nil {
			return v1.Errorf(http.StatusConflict, path.Base(p.To)+" is not there any more")
		}
		if err := d.FS.Carry(ctx, u, loc, ccfs.Modify); err != nil {
			return err
		}
		if _, err := d.FS.Entry(u, p.From, ccfs.Create); err != nil {
			return err
		}
		back[i] = renamePair{From: p.To, To: p.From}
	}
	return renamePairs(ctx, d, back)
}
