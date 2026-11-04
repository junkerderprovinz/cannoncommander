package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

var old = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

type env struct {
	t    *testing.T
	root string
	deps v1.Deps
	srv  *httptest.Server
	al   ArrowLoopConfig
}

var full = users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"fotos", "backups", "downloads"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	fsys, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, root: root}
	e.deps = v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t)}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		name := r.Header.Get("X-User")
		perms := full
		if name == "gast" {
			perms = users.SourceFilePermissions{View: true, Download: true}
		}
		return &users.User{Username: name, BackendScopes: []users.BackendScope{{Path: root, Scope: "/", Permissions: perms}}}, nil
	}
	r := v1.NewRouter(e.deps, auth)
	fileops.Routes(r)
	jobs.Routes(r)
	undo.Routes(r)
	trash.Routes(r)
	labels.Routes(r)
	r.Contract("tools", contract)
	bind(r, handlers{deps: e.deps, fs: fsys, meta: readMeta, arrowLoopConfig: func(context.Context) (ArrowLoopConfig, error) { return e.al, nil }})
	handler, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(handler)
	t.Cleanup(func() {
		e.srv.Close()
		if m := jobs.For(e.deps); m != nil {
			m.Close()
		}
		if s := trash.For(e.deps); s != nil {
			s.Close()
		}
		for _, name := range []string{"jobs", "trash", "fileops", "undo"} {
			v1.Forget(e.deps, name)
		}
	})
	return e
}

func (e *env) real(api string) string {
	return filepath.Join(e.root, filepath.FromSlash(strings.TrimPrefix(api, "/mnt/user")))
}

func (e *env) write(api, content string) {
	e.t.Helper()
	p := e.real(api)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
		e.t.Fatal(err)
	}
	if err := os.Chtimes(p, old, old); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(api string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.real(api))
	if err != nil {
		e.t.Fatalf("read %s: %v", api, err)
	}
	return string(b)
}

func (e *env) exists(api string) bool {
	_, err := os.Lstat(e.real(api))
	return err == nil
}

func (e *env) call(user, method, target string, body, out any) int {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.srv.URL+target, rd)
	req.Header.Set("X-User", user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s: %v in %s", method, target, err, raw)
		}
	}
	return resp.StatusCode
}

func (e *env) run(kind string, params any) jobs.Job {
	e.t.Helper()
	j := e.finish(kind, params)
	if j.State != jobs.Done {
		e.t.Fatalf("%s ended %s: %s %+v", kind, j.State, j.Error, j.Items)
	}
	return j
}

// finish starts a job and waits until it ended, however it ended.
func (e *env) finish(kind string, params any) jobs.Job {
	e.t.Helper()
	var j jobs.Job
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": kind, "params": params}, &j); code != http.StatusAccepted {
		e.t.Fatalf("start %s: %d %+v", kind, code, j)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		e.call("anna", "GET", "/jobs/"+j.ID, nil, &j)
		if !j.State.Open() {
			return j
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("job %s stuck in %s", j.ID, j.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func result[T any](t *testing.T, j jobs.Job) T {
	t.Helper()
	raw, _ := json.Marshal(j.Result)
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden file: %v (run with -update to create it)", err)
	}
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("%s differs from the golden file:\n%s", name, got)
	}
}

func checksumTree(e *env) {
	e.write("/mnt/user/fotos/Urlaub/strand.jpg", "sonne und meer")
	e.write("/mnt/user/fotos/Urlaub/2025/Grüße.txt", "Servus\n")
	e.write("/mnt/user/fotos/Urlaub/leer.txt", "")
	e.write("/mnt/user/fotos/Urlaub/alt.sha256", "nicht mitzählen")
}

func TestChecksumFilesMatchTheGoldenOnes(t *testing.T) {
	e := newEnv(t)
	checksumTree(e)
	for _, a := range Algos {
		for _, bsd := range []bool{false, true} {
			name := "urlaub." + a.ID
			if bsd {
				name = "urlaub-bsd." + a.ID
			}
			j := e.run(KindChecksum, ChecksumParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Algo: a.ID, Save: "/mnt/user/fotos/" + name, BSD: bsd})
			if r := result[struct{ Files []FileSum }](t, j); len(r.Files) != 3 {
				t.Errorf("%s: %d files, want 3 without the old list", a.ID, len(r.Files))
			}
			golden(t, name, e.read("/mnt/user/fotos/"+name))
		}
	}
}

func TestVerifyReportsEveryLine(t *testing.T) {
	e := newEnv(t)
	checksumTree(e)
	e.run(KindChecksum, ChecksumParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Save: "/mnt/user/fotos/urlaub.sha256"})
	e.write("/mnt/user/fotos/Urlaub/strand.jpg", "regen")
	os.Remove(e.real("/mnt/user/fotos/Urlaub/leer.txt"))
	list := e.read("/mnt/user/fotos/urlaub.sha256") + "das ist keine Zeile\n"
	e.write("/mnt/user/fotos/urlaub.sha256", list)

	j := e.run(KindVerify, VerifyParams{File: "/mnt/user/fotos/urlaub.sha256"})
	r := result[struct{ Lines []Checked }](t, j)
	states := map[string]string{}
	for _, l := range r.Lines {
		states[l.Name] = l.State
	}
	want := map[string]string{
		"Urlaub/2025/Grüße.txt": "ok", "Urlaub/strand.jpg": "mismatch", "Urlaub/leer.txt": "missing", "das ist keine Zeile": "bad",
	}
	for name, s := range want {
		if states[name] != s {
			t.Errorf("%s is %q, want %q (%v)", name, states[name], s, states)
		}
	}
}

func TestParseSumsReadsBothStyles(t *testing.T) {
	text := "# comment\n" +
		"d41d8cd98f00b204e9800998ecf8427e  leer.txt\n" +
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 *binär.bin\n" +
		"SHA256 (Ordner/mit Leer.txt) = E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855\n" +
		"\\e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  zeile\\nzwei\\\\x\n" +
		"kaputt\n"
	got := ParseSums(text, "SUMS")
	if len(got) != 5 {
		t.Fatalf("got %d lines: %+v", len(got), got)
	}
	if got[0].Algo != "md5" || got[1].Name != "binär.bin" || got[1].Algo != "sha256" {
		t.Errorf("GNU lines: %+v %+v", got[0], got[1])
	}
	if got[2].Name != "Ordner/mit Leer.txt" || got[2].Hash != strings.ToLower(got[2].Hash) {
		t.Errorf("BSD line: %+v", got[2])
	}
	if got[3].Name != "zeile\nzwei\\x" {
		t.Errorf("escaped name: %q", got[3].Name)
	}
	if got[4].Bad != "kaputt" {
		t.Errorf("bad line: %+v", got[4])
	}
	if b3 := ParseSums(got[1].Hash+"  x\n", "liste.b3"); b3[0].Algo != "b3" {
		t.Errorf("a .b3 list reads as %s", b3[0].Algo)
	}
	line := FormatLine(Algos[2], "ab", "zeile\nzwei\\x", false)
	if back := ParseSums(strings.Repeat("ab", 32)+line[3:], ""); len(back) != 1 {
		t.Errorf("an escaped line does not read back: %q", line)
	}
}

func TestSplitAndCombineGiveBackTheFile(t *testing.T) {
	e := newEnv(t)
	body := strings.Repeat("0123456789abcdef", 1<<16+100)
	e.write("/mnt/user/downloads/gross.bin", body)
	j := e.run(KindSplit, SplitParams{Path: "/mnt/user/downloads/gross.bin", Dest: "/mnt/user/backups", PartSize: 1 << 20, Sum: true})
	if r := result[struct{ Parts int }](t, j); r.Parts != 2 {
		t.Fatalf("parts %d", r.Parts)
	}
	if !e.exists("/mnt/user/backups/gross.bin.001") || !e.exists("/mnt/user/backups/gross.bin.002") || !e.exists("/mnt/user/backups/gross.bin.sha256") {
		t.Fatal("parts or the sum file are missing")
	}
	e.run(KindCombine, CombineParams{Path: "/mnt/user/backups/gross.bin.002", Dest: "/mnt/user/fotos", Check: true, Drop: true})
	if e.read("/mnt/user/fotos/gross.bin") != body {
		t.Fatal("the combined file differs")
	}
	if e.exists("/mnt/user/backups/gross.bin.001") {
		t.Error("the parts were not moved to the trash")
	}

	e.run(KindSplit, SplitParams{Path: "/mnt/user/downloads/gross.bin", Dest: "/mnt/user/backups", PartSize: 1 << 20, Sum: true})
	e.write("/mnt/user/backups/gross.bin.002", "falsch")
	j = e.finish(KindCombine, CombineParams{Path: "/mnt/user/backups/gross.bin.001", Dest: "/mnt/user/fotos", Check: true, Drop: true})
	if j.State != jobs.Failed || !e.exists("/mnt/user/backups/gross.bin.001") || e.exists("/mnt/user/fotos/gross (1).bin") {
		t.Errorf("a wrong checksum ended %s and left the parts %v", j.State, e.exists("/mnt/user/backups/gross.bin.001"))
	}
	os.Remove(e.real("/mnt/user/backups/gross.bin.002"))
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": KindCombine, "params": CombineParams{Path: "/mnt/user/backups/gross.bin.001", Dest: "/mnt/user/fotos"}}, nil); code != http.StatusBadRequest {
		t.Errorf("a missing part answered %d", code)
	}
}

func TestPartNamesGrowWithTheCount(t *testing.T) {
	if got := PartNames("a", 2500, 1000); !slices.Equal(got, []string{"a.001", "a.002", "a.003"}) {
		t.Errorf("got %v", got)
	}
	if got := PartNames("a", 1000*1000+1, 1000); got[0] != "a.0001" || len(got) != 1001 {
		t.Errorf("got %s and %d", got[0], len(got))
	}
}

func renameRows(t *testing.T, e *env, body BatchRequest) (int, BatchResult) {
	t.Helper()
	var out BatchResult
	code := e.call("anna", "POST", "/fs/rename/batch", body, &out)
	return code, out
}

func TestRenameByMask(t *testing.T) {
	e := newEnv(t)
	for _, n := range []string{"b.JPG", "a.jpg", "Notiz.txt"} {
		e.write("/mnt/user/fotos/Urlaub/"+n, n)
	}
	paths := []string{"/mnt/user/fotos/Urlaub/a.jpg", "/mnt/user/fotos/Urlaub/b.JPG", "/mnt/user/fotos/Urlaub/Notiz.txt"}
	code, out := renameRows(t, e, BatchRequest{Paths: paths, DryRun: true,
		Mask: &Mask{Name: "[P]_[C3]_[D]", Ext: "[E]", Start: 7, Step: 2, Case: "lower"}})
	if code != http.StatusOK || out.Done {
		t.Fatalf("dry run %d %+v", code, out)
	}
	got := []string{out.Rows[0].New, out.Rows[1].New, out.Rows[2].New}
	want := []string{"urlaub_007_2025-06-01.jpg", "urlaub_009_2025-06-01.jpg", "urlaub_011_2025-06-01.txt"}
	if !slices.Equal(got, want) {
		t.Errorf("names %v, want %v", got, want)
	}
	if !e.exists("/mnt/user/fotos/Urlaub/a.jpg") {
		t.Fatal("a dry run renamed")
	}

	_, out = renameRows(t, e, BatchRequest{Paths: paths[:2], Mask: &Mask{Name: "[N]", Ext: "[E]", Find: `^(\w)`, Repl: "x$1y", Regex: true, Case: "title"}})
	// The dot starts a word for title case, as in the prototype.
	if !out.Done || out.Rows[0].New != "Xay.Jpg" || out.Rows[1].New != "Xby.Jpg" || !e.exists("/mnt/user/fotos/Urlaub/Xay.Jpg") {
		t.Fatalf("regex rename %+v", out)
	}

	var undone map[string]any
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": out.Undo}, &undone); code != http.StatusOK {
		t.Fatalf("undo %d %v", code, undone)
	}
	if !e.exists("/mnt/user/fotos/Urlaub/a.jpg") || !e.exists("/mnt/user/fotos/Urlaub/b.JPG") {
		t.Error("undo did not bring the old names back")
	}
}

func TestRenameRefusesCollisionsAndSwapsNames(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/eins.txt", "1")
	e.write("/mnt/user/fotos/zwei.txt", "2")
	e.write("/mnt/user/fotos/drei.txt", "3")
	paths := []string{"/mnt/user/fotos/eins.txt", "/mnt/user/fotos/zwei.txt"}

	code, out := renameRows(t, e, BatchRequest{Paths: paths, Names: []string{"DREI.txt", "x/y"}})
	if code != http.StatusConflict || out.Rows[0].Error != "exists" || out.Rows[1].Error != "invalid" {
		t.Errorf("collisions %d %+v", code, out.Rows)
	}
	_, out = renameRows(t, e, BatchRequest{Paths: paths, Names: []string{"neu.txt", "NEU.txt"}, DryRun: true})
	if out.Rows[0].Error != "duplicate" || out.Rows[1].Error != "duplicate" {
		t.Errorf("duplicates %+v", out.Rows)
	}
	_, out = renameRows(t, e, BatchRequest{Paths: paths, Names: []string{"zwei.txt", "eins.txt"}})
	if !out.Done || e.read("/mnt/user/fotos/eins.txt") != "2" || e.read("/mnt/user/fotos/zwei.txt") != "1" {
		t.Errorf("swapping names %+v", out)
	}
	if code, _ := renameRows(t, e, BatchRequest{Paths: paths, Names: []string{"a"}}); code != http.StatusBadRequest {
		t.Errorf("too few names answered %d", code)
	}
}

func TestMaskTokensFromTheName(t *testing.T) {
	m := Mask{Name: "S[S2]E[F2] [T]", Ext: "[E]"}
	entries := []renameEntry{{name: "The.Show.s1e2.720p.mkv", info: fakeInfo{}}}
	names, missing, err := m.apply(entries, func(_, name string) map[byte]string { return readMeta("", name) })
	if err != nil {
		t.Fatal(err)
	}
	if names[0] != "S01E02.mkv" || !slices.Equal(missing[0], []string{"T"}) {
		t.Errorf("got %q missing %v", names[0], missing[0])
	}
	if goReplacement("$1x $& $$ ${2}") != "${1}x ${0} $$ ${2}" {
		t.Errorf("replacement %q", goReplacement("$1x $& $$ ${2}"))
	}
	if applyCase("hallo welt-und (mehr).TXT", "title") != "Hallo Welt-Und (Mehr).Txt" {
		t.Errorf("title case %q", applyCase("hallo welt-und (mehr).TXT", "title"))
	}
}

type fakeInfo struct{ os.FileInfo }

func (fakeInfo) ModTime() time.Time { return old }

func TestCompareAndSync(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/gleich.txt", "x")
	e.write("/mnt/user/backups/gleich.txt", "x")
	e.write("/mnt/user/fotos/nur-links.txt", "l")
	e.write("/mnt/user/backups/nur-rechts.txt", "r")
	e.write("/mnt/user/fotos/sub/neu.txt", "neuer")
	e.write("/mnt/user/backups/sub/neu.txt", "alt")
	later := old.Add(time.Hour)
	os.Chtimes(e.real("/mnt/user/fotos/sub/neu.txt"), later, later)

	var cmp struct {
		Items  []Compared
		Counts map[string]int
	}
	e.call("anna", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Recursive: true}, &cmp)
	if cmp.Counts[Same] != 1 || cmp.Counts[OnlyL] != 1 || cmp.Counts[OnlyR] != 1 || cmp.Counts[NewerL] != 1 {
		t.Fatalf("counts %v", cmp.Counts)
	}

	var plan struct{ Steps []Step }
	e.call("anna", "POST", "/tools/sync/plan", SyncPlanRequest{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Direction: "right", Recursive: true, Mirror: true}, &plan)
	acts := map[string]string{}
	for _, s := range plan.Steps {
		acts[s.Rel] = s.Act + s.To
	}
	if acts["nur-links.txt"] != "copyR" || acts["nur-rechts.txt"] != "deleteR" || acts["sub/neu.txt"] != "replaceR" {
		t.Fatalf("plan %+v", plan.Steps)
	}
	var steps []SyncStep
	for _, s := range plan.Steps {
		steps = append(steps, SyncStep{Rel: s.Rel, Act: s.Act, To: s.To})
	}
	e.run(KindSync, SyncParams{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Steps: steps})
	if e.read("/mnt/user/backups/sub/neu.txt") != "neuer" || e.read("/mnt/user/backups/nur-links.txt") != "l" || e.exists("/mnt/user/backups/nur-rechts.txt") {
		t.Fatal("sync did not make the sides agree")
	}
	if info, _ := os.Stat(e.real("/mnt/user/backups/sub/neu.txt")); !info.ModTime().Equal(later) {
		t.Errorf("the copy has the time %v", info.ModTime())
	}
	e.call("anna", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Recursive: true}, &cmp)
	if len(cmp.Items) != cmp.Counts[Same] {
		t.Errorf("after the sync the sides still differ: %v", cmp.Counts)
	}
	if code := e.call("gast", "POST", "/jobs", map[string]any{"kind": KindSync, "params": SyncParams{Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Steps: steps}}, nil); code != http.StatusForbidden {
		t.Errorf("a viewer could sync: %d", code)
	}
}

func TestPlanSwitchesOffWhatOverwritesNewer(t *testing.T) {
	items := []Compared{
		{Rel: "a", State: NewerR, Left: &Side{}, Right: &Side{}},
		{Rel: "b", State: Differ, Left: &Side{}, Right: &Side{}},
		{Rel: "c", State: NewerL, Left: &Side{}, Right: &Side{}},
	}
	steps := Plan(items, "right", false)
	if !steps[0].Off || steps[0].Why != "rightNewer" || !steps[1].Off || steps[1].Why != "sameAge" || steps[2].Off {
		t.Errorf("steps %+v", steps)
	}
	if both := Plan(items, "both", false); both[0].To != "L" || both[2].To != "R" || !both[1].Off {
		t.Errorf("both %+v", both)
	}
}

func TestNaturalOrder(t *testing.T) {
	names := []string{"Datei10", "datei2", "Datei1", "b", "A"}
	slices.SortFunc(names, naturalCompare)
	if !slices.Equal(names, []string{"A", "b", "Datei1", "datei2", "Datei10"}) {
		t.Errorf("order %v", names)
	}
}

func TestDiffPairsChangedLines(t *testing.T) {
	a := []string{"eins", "zwei", "drei", "vier", "fünf"}
	b := []string{"eins", "2", "drei", "vier", "fünf", "sechs"}
	rows, err := DiffLines(a, b)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, r := range rows {
		kinds = append(kinds, r.T)
	}
	if !slices.Equal(kinds, []string{"eq", "chg", "eq", "eq", "eq", "add"}) {
		t.Errorf("rows %v", kinds)
	}

	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", strings.Join(a, "\n")+"\n")
	e.write("/mnt/user/fotos/b.txt", strings.Join(b, "\r\n"))
	var out struct {
		Hunks  int
		Binary bool
		Rows   []DiffRow
	}
	e.call("anna", "POST", "/tools/diff", map[string]string{"left": "/mnt/user/fotos/a.txt", "right": "/mnt/user/fotos/b.txt"}, &out)
	if out.Hunks != 2 || out.Binary || len(out.Rows) != 6 {
		t.Errorf("diff %+v", out)
	}
	e.write("/mnt/user/fotos/c.bin", "\x00\x01abc")
	e.call("anna", "POST", "/tools/diff", map[string]string{"left": "/mnt/user/fotos/c.bin", "right": "/mnt/user/fotos/a.txt"}, &out)
	if !out.Binary {
		t.Error("a file with NUL bytes was compared as text")
	}
}

func TestCleanupFindsTheUsualLeftovers(t *testing.T) {
	e := newEnv(t)
	base := "/mnt/user/downloads/unfertig/"
	if err := os.MkdirAll(e.real(base+"alt/leer"), 0o777); err != nil {
		t.Fatal(err)
	}
	e.write(base+".DS_Store", "x")
	e.write(base+"film.mkv.!qB", "x")
	e.write(base+"~$Bewerbung.docx", "x")
	e.write(base+"notizen-leer.txt", "")
	e.write(base+"Urlaub-007.jpg", "\x89PNG\r\n\x1a\nrest")
	if runtime.GOOS != "windows" {
		e.write(base+"Rechnung März.pdf ", "%PDF-1.7")
		e.write(base+"Notiz: wichtig?.txt", "x")
	}
	e.write(base+"ok.txt", "x")
	if runtime.GOOS != "windows" {
		os.Symlink("/mnt/user/isos/weg.iso", e.real(base+"altes-iso"))
	}
	var scan Scan
	if code := e.call("anna", "POST", "/tools/cleanup/scan", map[string]string{"path": "/mnt/user/downloads"}, &scan); code != http.StatusOK {
		t.Fatalf("scan %d", code)
	}
	count := func(g string) int { return len(scan.Groups[g]) }
	if count(GroupEmptyDir) != 1 || scan.Groups[GroupEmptyDir][0].Note != "onlyEmptyDirs" {
		t.Errorf("empty folders %+v", scan.Groups[GroupEmptyDir])
	}
	if count(GroupTemp) != 3 || count(GroupEmptyFile) != 1 {
		t.Errorf("temp %+v, empty %+v", scan.Groups[GroupTemp], scan.Groups[GroupEmptyFile])
	}
	if count(GroupExt) != 1 || scan.Groups[GroupExt][0].To != "Urlaub-007.png" {
		t.Errorf("endings %+v", scan.Groups[GroupExt])
	}
	if runtime.GOOS != "windows" {
		if count(GroupName) != 2 {
			t.Errorf("names %+v", scan.Groups[GroupName])
		}
		if count(GroupLink) != 1 {
			t.Errorf("links %+v", scan.Groups[GroupLink])
		}
	}
}

func TestPublicAddresses(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.215.14": true, "2606:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "192.168.10.10": false, "172.20.0.1": false, "169.254.169.254": false,
		"100.64.1.1": false, "0.0.0.0": false, "::1": false, "fd00::1": false, "fe80::1": false, "::ffff:127.0.0.1": false,
		"224.0.0.1": false,
	} {
		if got := PublicAddr(mustAddr(t, addr)); got != want {
			t.Errorf("PublicAddr(%s) = %v", addr, got)
		}
	}
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFetchRefusesTheLocalNetwork(t *testing.T) {
	e := newEnv(t)
	inside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "geheim") }))
	defer inside.Close()
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": KindFetch, "params": FetchParams{URL: inside.URL + "/x", Dest: "/mnt/user/downloads"}}, nil); code != http.StatusBadRequest {
		t.Errorf("a loopback address was accepted: %d", code)
	}
	host := strings.Replace(inside.URL, "127.0.0.1", "localhost", 1)
	j := e.finish(KindFetch, FetchParams{URL: host + "/x", Dest: "/mnt/user/downloads"})
	if j.State != jobs.Failed || !strings.Contains(j.Error, "local network") || e.exists("/mnt/user/downloads/x") {
		t.Errorf("a name for loopback ended %s: %s", j.State, j.Error)
	}
	for _, bad := range []string{"ftp://example.org/x", "file:///etc/passwd", "https://user:pw@example.org/x", "kein url"} {
		if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": KindFetch, "params": FetchParams{URL: bad, Dest: "/mnt/user/downloads"}}, nil); code != http.StatusBadRequest {
			t.Errorf("%s answered %d", bad, code)
		}
	}
}

func TestFetchLoadsIntoTheFolder(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/umweg" {
			http.Redirect(w, r, "/datei.iso", http.StatusFound)
			return
		}
		w.Header().Set("Last-Modified", old.Format(http.TimeFormat))
		io.WriteString(w, strings.Repeat("iso", 1000))
	}))
	defer srv.Close()
	// The test server listens on loopback, which the guard refuses, so the
	// public name is sent there by a transport of its own.
	saved := fetchClient.Transport
	fetchClient.Transport = &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}}
	defer func() { fetchClient.Transport = saved }()
	e.write("/mnt/user/downloads/datei.iso", "alt")

	j := e.run(KindFetch, FetchParams{URL: "http://example.org/umweg", Dest: "/mnt/user/downloads"})
	if got := result[map[string]string](t, j)["path"]; got != "/mnt/user/downloads/umweg" {
		t.Errorf("saved as %s", got)
	}
	if j.Name != "example.org/umweg" {
		t.Errorf("the job is called %q, it should say where the file comes from", j.Name)
	}
	if e.read("/mnt/user/downloads/umweg") != strings.Repeat("iso", 1000) || e.read("/mnt/user/downloads/datei.iso") != "alt" {
		t.Error("the content is wrong or the old file was replaced")
	}
	if info, _ := os.Stat(e.real("/mnt/user/downloads/umweg")); !info.ModTime().Equal(old) {
		t.Errorf("time %v", info.ModTime())
	}
	j = e.run(KindFetch, FetchParams{URL: "http://example.org/datei.iso", Dest: "/mnt/user/downloads"})
	if got := result[map[string]string](t, j)["path"]; got != "/mnt/user/downloads/datei (1).iso" {
		t.Errorf("a taken name was saved as %s", got)
	}
}

func TestArrowLoopGetsTheJob(t *testing.T) {
	e := newEnv(t)
	var got map[string]any
	var token string
	al := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get("X-ArrowLoop-Token")
		json.NewDecoder(r.Body).Decode(&got)
		if got["name"] == "doppelt" {
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, `{"error":"there is a job called \"doppelt\" already"}`)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(got)
	}))
	defer al.Close()
	e.al = ArrowLoopConfig{URL: al.URL, Token: "geheim", Paths: map[string]string{"/mnt/user/": "/data"}}

	req := ArrowLoopRequest{Name: "Fotos nach Backup", Left: "/mnt/user/fotos", Right: "/mnt/user/backups", Direction: "right", Mirror: true, Every: "daily", At: "03:15"}
	if code := e.call("anna", "POST", "/tools/arrowloop", req, nil); code != http.StatusCreated {
		t.Fatalf("hand over answered %d", code)
	}
	want := map[string]any{"name": "Fotos nach Backup", "left": "/data/fotos", "right": "/data/backups", "direction": "leftToRight", "mode": "mirror", "schedule": "15 3 * * *"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s is %v, want %v", k, got[k], v)
		}
	}
	if token != "geheim" {
		t.Errorf("token %q", token)
	}
	req.Name = "doppelt"
	if code := e.call("anna", "POST", "/tools/arrowloop", req, nil); code != http.StatusConflict {
		t.Errorf("a taken name answered %d", code)
	}
	req.Name = "gast"
	if code := e.call("gast", "POST", "/tools/arrowloop", req, nil); code != http.StatusForbidden {
		t.Errorf("a viewer could plan a mirror: %d", code)
	}
	e.al.URL = ""
	if code := e.call("anna", "POST", "/tools/arrowloop", req, nil); code != http.StatusServiceUnavailable {
		t.Errorf("without ArrowLoop the answer is %d", code)
	}
}

func TestCronFromTheDialog(t *testing.T) {
	for _, c := range []struct{ every, at, want string }{
		{"hourly", "03:15", "15 * * * *"}, {"daily", "", "0 3 * * *"}, {"weekly", "22:05", "5 22 * * 1"},
	} {
		if got, err := Cron(c.every, c.at); err != nil || got != c.want {
			t.Errorf("Cron(%s, %s) = %q, %v", c.every, c.at, got, err)
		}
	}
	if _, err := Cron("daily", "25:00"); err == nil {
		t.Error("25:00 was taken")
	}
}

func TestArchiveJobs(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "sonne")
	e.write("/mnt/user/fotos/Urlaub/b/c.txt", "meer")
	for _, f := range []string{"zip", "tar.zst", "tar.gz"} {
		j := e.run(KindPack, PackParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups", Name: "urlaub", Format: f})
		out := result[map[string]string](t, j)["path"]
		if out != "/mnt/user/backups/urlaub."+f {
			t.Fatalf("packed to %s", out)
		}
		e.run(KindTestArchive, TestArchiveParams{Paths: []string{out}})
		e.run(KindUnpack, UnpackParams{Archives: []string{out}, Dest: "/mnt/user/downloads", Own: true})
		if e.read("/mnt/user/downloads/urlaub/Urlaub/b/c.txt") != "meer" {
			t.Fatalf("%s did not unpack", f)
		}
		os.RemoveAll(e.real("/mnt/user/downloads/urlaub"))
	}

	j := e.run(KindUnpack, UnpackParams{Archives: []string{"/mnt/user/backups/urlaub.zip::/Urlaub"}, Items: []string{"b"}, Dest: "/mnt/user/downloads"})
	if e.read("/mnt/user/downloads/b/c.txt") != "meer" || e.exists("/mnt/user/downloads/a.jpg") {
		t.Errorf("unpacking a pick went wrong: %+v", j.Result)
	}

	e.run(KindPack, PackParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups", Name: "geheim", Format: "zip", Password: "Bergkäse"})
	j = e.finish(KindTestArchive, TestArchiveParams{Paths: []string{"/mnt/user/backups/geheim.zip"}})
	r := result[struct{ Archives []TestResult }](t, j)
	if len(r.Archives) != 1 || r.Archives[0].OK || !r.Archives[0].Password {
		t.Errorf("an archive without its password tested as %+v", r.Archives)
	}
	var info ArchiveInfo
	if code := e.call("anna", "GET", "/tools/archive?path=/mnt/user/backups/geheim.zip", nil, &info); code != http.StatusOK || !info.Encrypted || info.Unpacked != 9 {
		t.Errorf("info %d %+v", code, info)
	}

	e.write("/mnt/user/backups/kaputt.zip", "PK\x03\x04 das ist kein zip")
	j = e.finish(KindTestArchive, TestArchiveParams{Paths: []string{"/mnt/user/backups/kaputt.zip"}})
	if r := result[struct{ Archives []TestResult }](t, j); r.Archives[0].OK || r.Archives[0].Damage == nil {
		t.Errorf("a broken archive tested as %+v", r.Archives)
	}
	if code := e.call("gast", "POST", "/jobs", map[string]any{"kind": KindPack, "params": PackParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups", Name: "x", Format: "zip"}}, nil); code != http.StatusForbidden {
		t.Errorf("a viewer could pack: %d", code)
	}
}

func TestLinksAndRights(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links and modes work differently on Windows")
	}
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "a")
	var out map[string]string
	if code := e.call("anna", "POST", "/fs/link", LinkRequest{Target: "/mnt/user/fotos/a.txt", Path: "/mnt/user/backups/a-link"}, &out); code != http.StatusCreated {
		t.Fatalf("symlink %d", code)
	}
	if e.read("/mnt/user/backups/a-link") != "a" {
		t.Error("the link does not lead to the file")
	}
	if code := e.call("anna", "POST", "/fs/link", LinkRequest{Target: "/etc/passwd", Path: "/mnt/user/backups/raus"}, nil); code == http.StatusCreated {
		t.Error("a link out of the sources was made")
	}
	if code := e.call("anna", "POST", "/fs/link", LinkRequest{Target: "/mnt/user/fotos/a.txt", Path: "/mnt/user/fotos/a-hart", Hard: true}, nil); code != http.StatusCreated {
		t.Errorf("hard link %d", code)
	}
	a, _ := os.Stat(e.real("/mnt/user/fotos/a.txt"))
	b, _ := os.Stat(e.real("/mnt/user/fotos/a-hart"))
	if !os.SameFile(a, b) {
		t.Error("the hard link is a copy")
	}
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": out["undo"]}, nil); code != http.StatusOK || e.exists("/mnt/user/backups/a-link") {
		t.Errorf("undo of the link %d", code)
	}

	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/a.txt"}, Mode: "640"}, nil); code != http.StatusOK {
		t.Fatalf("attrs %d", code)
	}
	if info, _ := os.Stat(e.real("/mnt/user/fotos/a.txt")); info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", info.Mode())
	}
	e.write("/mnt/user/fotos/tief/x/y.txt", "y")
	var j jobs.Job
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/tief"}, Mode: "0750", Recursive: true}, &j); code != http.StatusAccepted {
		t.Fatalf("recursive attrs %d", code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for j.State.Open() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		e.call("anna", "GET", "/jobs/"+j.ID, nil, &j)
	}
	if info, _ := os.Stat(e.real("/mnt/user/fotos/tief/x/y.txt")); info.Mode().Perm() != 0o750 {
		t.Errorf("the file deep inside has %v", info.Mode())
	}
	if code := e.call("anna", "PATCH", "/fs/attrs", AttrsParams{Paths: []string{"/mnt/user/fotos/a.txt"}, Mode: "9"}, nil); code != http.StatusBadRequest {
		t.Errorf("a bad mode answered %d", code)
	}
}

func TestEveryJobKindIsKnown(t *testing.T) {
	e := newEnv(t)
	for _, k := range []string{KindPack, KindUnpack, KindTestArchive, KindChecksum, KindVerify, KindSplit, KindCombine, KindAttrs, KindSync, KindFetch} {
		if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": k, "params": map[string]any{}}, nil); code == http.StatusBadRequest {
			var p v1.Problem
			e.call("anna", "POST", "/jobs", map[string]any{"kind": k, "params": map[string]any{}}, &p)
			if p.Detail == "unknown job kind" {
				t.Errorf("%s is not registered", k)
			}
		}
	}
}
