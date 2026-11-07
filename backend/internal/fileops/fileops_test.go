package fileops_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/testutil"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type env struct {
	t     *testing.T
	root  string
	roots []string
	deps  v1.Deps
	srv   *httptest.Server
	rules *pathRules
	// demoted takes the admin right from the admin.
	demoted atomic.Bool
}

// pathRules is a rule book for every user alike: a path with its effect covers
// what lies below it.
type pathRules struct {
	mu    sync.Mutex
	paths map[string]ccfs.Effect
	names map[string]bool
}

// hideName hides a name everywhere, the kind of rule that lets a folder go.
func (r *pathRules) hideName(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names == nil {
		r.names = map[string]bool{}
	}
	r.names[name] = true
}

func (r *pathRules) CarryEffect(_ *users.User, p string) ccfs.Effect {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.onPath(p)
}

func (r *pathRules) CarryBelow(*users.User, string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paths) > 0
}

func (r *pathRules) set(p string, effect ccfs.Effect) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths[p] = effect
}

func (r *pathRules) Effect(_ *users.User, p string) ccfs.Effect {
	r.mu.Lock()
	defer r.mu.Unlock()
	if effect := r.onPath(p); effect != ccfs.Open {
		return effect
	}
	if r.names[path.Base(p)] {
		return ccfs.Hidden
	}
	return ccfs.Open
}

func (r *pathRules) onPath(p string) ccfs.Effect {
	for at, effect := range r.paths {
		if p == at || strings.HasPrefix(p, at+"/") {
			return effect
		}
	}
	return ccfs.Open
}

var full = users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true}

// newEnv serves the B packages over a share source at /mnt/user with the
// shares fotos and backups, plus any extra sources.
func newEnv(t *testing.T, extra ...ccfs.Source) *env {
	t.Helper()
	return newEnvWith(t, nil, extra...)
}

// newEnvWith is newEnv with a resolver that wrap builds from the plain one.
func newEnvWith(t *testing.T, wrap func(*ccfs.Resolver) *ccfs.Resolver, extra ...ccfs.Source) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"fotos", "backups"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	e := &env{t: t, root: root, rules: &pathRules{paths: map[string]ccfs.Effect{}}}
	sources := append([]ccfs.Source{{Name: "Shares", Mount: "/mnt/user", Root: root}}, extra...)
	fsys, err := ccfs.New(e.rules, sources...)
	if err != nil {
		t.Fatal(err)
	}
	if wrap != nil {
		fsys = wrap(fsys)
	}
	for _, s := range sources {
		e.roots = append(e.roots, s.Root)
	}
	e.deps = v1.Deps{FS: fsys, Hub: hub.New(hub.Options{}), DB: testutil.MainDB(t), Users: e.user}
	e.serve()
	return e
}

func (e *env) serve() {
	t := e.t
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		return e.user(r.Header.Get("X-User"))
	}
	r := v1.NewRouter(e.deps, auth)
	fileops.Routes(r)
	jobs.Routes(r)
	undo.Routes(r)
	trash.Routes(r)
	labels.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(h)
	t.Cleanup(e.stop)
}

func (e *env) user(name string) (*users.User, error) {
	perms := full
	if name == "gast" {
		perms = users.SourceFilePermissions{View: true, Download: true}
	}
	u := &users.User{}
	for _, root := range e.roots {
		u.BackendScopes = append(u.BackendScopes, users.BackendScope{Path: root, Scope: "/", Permissions: perms})
	}
	u.Username = name
	u.Permissions.Admin = name == "admin" && !e.demoted.Load()
	return u, nil
}

// stop shuts everything down as the server would on exit.
func (e *env) stop() {
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
}

func (e *env) real(api string) string {
	return filepath.Join(e.root, filepath.FromSlash(strings.TrimPrefix(api, "/mnt/user")))
}

func (e *env) write(api, content string, mtime time.Time) {
	e.t.Helper()
	p := e.real(api)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
		e.t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			e.t.Fatal(err)
		}
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

func (e *env) gone(api string) bool {
	_, err := os.Lstat(e.real(api))
	return os.IsNotExist(err)
}

// call sends a request as user and decodes the answer into out.
func (e *env) call(user, method, target string, body, out any) int {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
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
	if out != nil && resp.StatusCode < 300 && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s: %v in %s", method, target, err, raw)
		}
	}
	return resp.StatusCode
}

func (e *env) start(kind string, params any) jobs.Job {
	e.t.Helper()
	var j jobs.Job
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": kind, "params": params}, &j); code != http.StatusAccepted {
		e.t.Fatalf("start %s: %d", kind, code)
	}
	return j
}

// wait polls the job until it ended or waits for an answer.
func (e *env) wait(id string) jobs.Job {
	e.t.Helper()
	return e.until(id, func(j jobs.Job) bool { return !j.State.Open() || j.State == jobs.Asking })
}

// ended polls the job until it has stopped. A cancelled job that waits for
// an answer still asks until its run has returned.
func (e *env) ended(id string) jobs.Job {
	e.t.Helper()
	return e.until(id, func(j jobs.Job) bool { return !j.State.Open() })
}

func (e *env) until(id string, done func(jobs.Job) bool) jobs.Job {
	e.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var j jobs.Job
		e.call("anna", "GET", "/jobs/"+id, nil, &j)
		if done(j) {
			return j
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("job %s stuck in %s", id, j.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (e *env) run(kind string, params any) jobs.Job {
	e.t.Helper()
	j := e.wait(e.start(kind, params).ID)
	if j.State != jobs.Done {
		e.t.Fatalf("%s ended %s: %s %+v", kind, j.State, j.Error, j.Items)
	}
	return j
}

func (e *env) undo(id string) {
	e.t.Helper()
	var out struct {
		Job *jobs.Job `json:"job"`
	}
	code := e.call("anna", "POST", "/undo", map[string]string{"id": id}, &out)
	if code != http.StatusOK && code != http.StatusAccepted {
		e.t.Fatalf("undo: %d", code)
	}
	if out.Job != nil {
		if j := e.wait(out.Job.ID); j.State != jobs.Done {
			e.t.Fatalf("undo job ended %s: %s %+v", j.State, j.Error, j.Items)
		}
	}
}

func (e *env) trashItems() []trash.Item {
	e.t.Helper()
	var out struct {
		Items []trash.Item `json:"items"`
	}
	e.call("anna", "GET", "/trash", nil, &out)
	return out.Items
}

var old = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

func TestCopyKeepsContentAndTimesAndCanBeTakenBack(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "sonne", old)
	e.write("/mnt/user/fotos/Urlaub/2025/b.jpg", "strand", old)
	e.write("/mnt/user/fotos/notiz.txt", "hallo", old)

	j := e.run("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Urlaub", "/mnt/user/fotos/notiz.txt"}, Dest: "/mnt/user/backups"})
	if e.read("/mnt/user/backups/Urlaub/2025/b.jpg") != "strand" || e.read("/mnt/user/backups/notiz.txt") != "hallo" {
		t.Fatal("content differs")
	}
	if info, _ := os.Stat(e.real("/mnt/user/backups/Urlaub/a.jpg")); !info.ModTime().Equal(old) {
		t.Fatalf("mtime %s", info.ModTime())
	}
	if j.DoneBytes != j.TotalBytes || j.TotalBytes != int64(len("sonnestrandhallo")) {
		t.Fatalf("progress %d of %d", j.DoneBytes, j.TotalBytes)
	}
	if e.read("/mnt/user/fotos/notiz.txt") != "hallo" {
		t.Fatal("the source changed")
	}
	if j.Undo == "" {
		t.Fatal("no undo step")
	}
	e.undo(j.Undo)
	if !e.gone("/mnt/user/backups/Urlaub") || !e.gone("/mnt/user/backups/notiz.txt") || e.gone("/mnt/user/fotos/Urlaub/a.jpg") {
		t.Fatal("undo did not remove exactly the copies")
	}
}

func TestCopyWithVerifyChecksEveryFile(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/big.bin", strings.Repeat("x", 3*jobs.Chunk+17), old)
	e.run("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/big.bin"}, Dest: "/mnt/user/backups", Verify: true})
	if len(e.read("/mnt/user/backups/big.bin")) != 3*jobs.Chunk+17 {
		t.Fatal("size differs")
	}
	entries, _ := os.ReadDir(e.real("/mnt/user/backups"))
	for _, d := range entries {
		if strings.HasSuffix(d.Name(), fileops.PartSuffix) {
			t.Fatalf("part file left: %s", d.Name())
		}
	}
}

func TestConflictRules(t *testing.T) {
	newer := old.Add(time.Hour)
	for _, tc := range []struct {
		rule  string
		src   time.Time
		want  string
		extra string
	}{
		{rule: "skip", src: newer, want: "alt"},
		{rule: "replace", src: old.Add(-time.Hour), want: "neu"},
		{rule: "both", src: newer, want: "alt", extra: "a (1).txt"},
		{rule: "newer", src: newer, want: "neu"},
		{rule: "newer", src: old.Add(-time.Hour), want: "alt"},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			e := newEnv(t)
			e.write("/mnt/user/fotos/a.txt", "neu", tc.src)
			e.write("/mnt/user/backups/a.txt", "alt", old)
			e.run("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.txt"}, Dest: "/mnt/user/backups", Rule: tc.rule})
			if got := e.read("/mnt/user/backups/a.txt"); got != tc.want {
				t.Fatalf("a.txt holds %q", got)
			}
			if tc.extra != "" && e.read("/mnt/user/backups/"+tc.extra) != "neu" {
				t.Fatal("no second copy")
			}
		})
	}
}

func TestReplaceSendsTheOldFileToTheTrashAndUndoBringsItBack(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "neu", time.Time{})
	e.write("/mnt/user/backups/a.txt", "alt", old)
	j := e.run("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.txt"}, Dest: "/mnt/user/backups", Rule: "replace"})
	items := e.trashItems()
	if len(items) != 1 || items[0].Origin != "/mnt/user/backups/a.txt" {
		t.Fatalf("trash holds %+v", items)
	}
	e.undo(j.Undo)
	if got := e.read("/mnt/user/backups/a.txt"); got != "alt" {
		t.Fatalf("after undo a.txt holds %q", got)
	}
	if len(e.trashItems()) != 0 {
		t.Fatal("the old file is still in the trash")
	}
}

func TestAskedConflictWaitsForTheDecision(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "neu", time.Time{})
	e.write("/mnt/user/backups/a.txt", "alt", old)
	j := e.wait(e.start("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.txt"}, Dest: "/mnt/user/backups"}).ID)
	if j.State != jobs.Asking || j.Conflict == nil || j.Conflict.Key != "a.txt" || j.Conflict.Newer != "source" {
		t.Fatalf("job: %s %+v", j.State, j.Conflict)
	}
	if code := e.call("gast", "POST", "/jobs/"+j.ID+"/decide", map[string]string{"choice": "replace"}, nil); code != http.StatusNotFound {
		t.Fatalf("another user answered: %d", code)
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/decide", map[string]string{"choice": "both"}, nil); code != http.StatusOK {
		t.Fatalf("decide: %d", code)
	}
	if j = e.wait(j.ID); j.State != jobs.Done {
		t.Fatalf("ended %s", j.State)
	}
	if e.read("/mnt/user/backups/a.txt") != "alt" || e.read("/mnt/user/backups/a (1).txt") != "neu" {
		t.Fatal("both should be kept")
	}
}

func TestFoldersAreMergedAndADecisionCoversTheirContent(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "neu a", time.Time{})
	e.write("/mnt/user/fotos/Urlaub/b.jpg", "neu b", time.Time{})
	e.write("/mnt/user/backups/Urlaub/a.jpg", "alt a", old)
	e.write("/mnt/user/backups/Urlaub/c.jpg", "alt c", old)
	e.run("copy", fileops.TransferParams{
		Sources:   []string{"/mnt/user/fotos/Urlaub"},
		Dest:      "/mnt/user/backups",
		Decisions: map[string]string{"Urlaub": "replace"},
	})
	for p, want := range map[string]string{"a.jpg": "neu a", "b.jpg": "neu b", "c.jpg": "alt c"} {
		if got := e.read("/mnt/user/backups/Urlaub/" + p); got != want {
			t.Errorf("%s holds %q", p, got)
		}
	}
}

func TestMoveRenamesAndUndoMovesBack(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	var moved [][2]string
	stop := fschange.Observe(fschange.Observer{Moved: func(from, to string) {
		mu.Lock()
		moved = append(moved, [2]string{from, to})
		mu.Unlock()
	}})
	defer stop()
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "sonne", old)
	j := e.run("move", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups"})
	if !e.gone("/mnt/user/fotos/Urlaub") || e.read("/mnt/user/backups/Urlaub/a.jpg") != "sonne" {
		t.Fatal("not moved")
	}
	mu.Lock()
	if len(moved) != 1 || moved[0] != [2]string{"/mnt/user/fotos/Urlaub", "/mnt/user/backups/Urlaub"} {
		t.Fatalf("observer heard %v", moved)
	}
	mu.Unlock()
	e.undo(j.Undo)
	if e.gone("/mnt/user/fotos/Urlaub/a.jpg") || !e.gone("/mnt/user/backups/Urlaub") {
		t.Fatal("undo did not move it back")
	}
}

func TestMoveMergesIntoAnExistingFolder(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "neu a", time.Time{})
	e.write("/mnt/user/backups/Urlaub/c.jpg", "alt c", old)
	j := e.run("move", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/backups", Rule: "replace"})
	if !e.gone("/mnt/user/fotos/Urlaub") || e.read("/mnt/user/backups/Urlaub/a.jpg") != "neu a" || e.read("/mnt/user/backups/Urlaub/c.jpg") != "alt c" {
		t.Fatal("not merged")
	}
	e.undo(j.Undo)
	if e.read("/mnt/user/fotos/Urlaub/a.jpg") != "neu a" || !e.gone("/mnt/user/backups/Urlaub/a.jpg") || e.read("/mnt/user/backups/Urlaub/c.jpg") != "alt c" {
		t.Fatal("undo of the merge went wrong")
	}
}

func TestCopyIntoItselfOrOntoItselfIsRefused(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "x", time.Time{})
	for _, p := range []fileops.TransferParams{
		{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/fotos/Urlaub"},
		{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/fotos"},
		{Sources: []string{"/mnt/user/fotos"}, Dest: "/mnt/user/fotos/Urlaub"},
		{Sources: []string{"/mnt/user/fotos/Urlaub"}, Dest: "/mnt/user/fotos/Urlaub/a.jpg"},
	} {
		if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": "copy", "params": p}, nil); code != http.StatusBadRequest {
			t.Errorf("%v into %s: %d", p.Sources, p.Dest, code)
		}
	}
}

func TestACopyCutOffByARestartGoesOnFromItsPartFile(t *testing.T) {
	e := newEnv(t)
	content := strings.Repeat("0123456789abcdef", 4*jobs.Chunk/16)
	e.write("/mnt/user/fotos/film.mkv", content, old)
	var j jobs.Job
	params := map[string]any{"kind": "copy", "cap": 2 * jobs.Chunk,
		"params": fileops.TransferParams{Sources: []string{"/mnt/user/fotos/film.mkv"}, Dest: "/mnt/user/backups"}}
	if code := e.call("anna", "POST", "/jobs", params, &j); code != http.StatusAccepted {
		t.Fatalf("start: %d", code)
	}
	part := e.real("/mnt/user/backups/.film.mkv" + fileops.PartSuffix)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if info, err := os.Stat(part); err == nil && info.Size() >= 2*jobs.Chunk {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the copy did not get going")
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.stop()
	// A mark in the part file shows whether the next run goes on from it
	// or starts over.
	f, err := os.OpenFile(part, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("shutdown removed the part file: %v", err)
	}
	if _, err := f.WriteAt([]byte("Z"), 0); err != nil {
		t.Fatal(err)
	}
	f.Close()
	e.serve()
	done := e.wait(j.ID)
	got := e.read("/mnt/user/backups/film.mkv")
	if done.State != jobs.Done || got != "Z"+content[1:] {
		t.Fatalf("after the restart: %s %s, started over: %v", done.State, done.Error, got == content)
	}
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatal("part file left")
	}
}

func TestCancelRemovesTheHalfWrittenFile(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/film.mkv", strings.Repeat("x", 4*jobs.Chunk), old)
	var j jobs.Job
	params := map[string]any{"kind": "copy", "cap": jobs.Chunk,
		"params": fileops.TransferParams{Sources: []string{"/mnt/user/fotos/film.mkv"}, Dest: "/mnt/user/backups"}}
	e.call("anna", "POST", "/jobs", params, &j)
	part := e.real("/mnt/user/backups/.film.mkv" + fileops.PartSuffix)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, err := os.Stat(part); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no part file")
		}
	}
	if code := e.call("anna", "POST", "/jobs/"+j.ID+"/cancel", nil, nil); code != http.StatusOK {
		t.Fatalf("cancel: %d", code)
	}
	if j = e.wait(j.ID); j.State != jobs.Cancelled {
		t.Fatalf("ended %s", j.State)
	}
	if !e.gone("/mnt/user/backups/.film.mkv"+fileops.PartSuffix) || !e.gone("/mnt/user/backups/film.mkv") {
		t.Fatal("cancel left files behind")
	}
}

func TestTrashRestoreAndUndo(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/2025/a.jpg", "sonne", old)
	e.write("/mnt/user/fotos/b.jpg", "regen", old)
	j := e.run("trash", trash.PathsParams{Paths: []string{"/mnt/user/fotos/2025", "/mnt/user/fotos/b.jpg"}})
	if !e.gone("/mnt/user/fotos/2025") || !e.gone("/mnt/user/fotos/b.jpg") {
		t.Fatal("still there")
	}
	items := e.trashItems()
	if len(items) != 2 || items[0].By != "anna" {
		t.Fatalf("trash: %+v", items)
	}
	for _, it := range items {
		if !strings.HasPrefix(e.real(it.Origin), e.root) {
			t.Fatal("origin outside the source")
		}
	}
	if entries, _ := os.ReadDir(e.real("/mnt/user/fotos/" + trash.DirName)); len(entries) != 2 {
		t.Fatalf("bin of the share holds %d entries", len(entries))
	}

	e.undo(j.Undo)
	if e.read("/mnt/user/fotos/2025/a.jpg") != "sonne" || e.read("/mnt/user/fotos/b.jpg") != "regen" || len(e.trashItems()) != 0 {
		t.Fatal("undo did not restore")
	}

	e.run("trash", trash.PathsParams{Paths: []string{"/mnt/user/fotos/b.jpg"}})
	e.write("/mnt/user/fotos/b.jpg", "neu", time.Time{})
	var restore jobs.Job
	if code := e.call("anna", "POST", "/trash/restore", map[string]any{"ids": []string{e.trashItems()[0].ID}}, &restore); code != http.StatusAccepted {
		t.Fatalf("restore: %d", code)
	}
	if e.wait(restore.ID).State != jobs.Done {
		t.Fatal("restore failed")
	}
	if e.read("/mnt/user/fotos/b (1).jpg") != "regen" || e.read("/mnt/user/fotos/b.jpg") != "neu" {
		t.Fatal("a taken name should get a number")
	}
}

func TestEmptyingTheTrashDeletesForGood(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", old)
	e.write("/mnt/user/backups/b.jpg", "b", old)
	e.run("trash", trash.PathsParams{Paths: []string{"/mnt/user/fotos/a.jpg", "/mnt/user/backups/b.jpg"}})
	items := e.trashItems()
	var j jobs.Job
	if code := e.call("anna", "DELETE", "/trash/"+items[0].ID, nil, &j); code != http.StatusAccepted || e.wait(j.ID).State != jobs.Done {
		t.Fatalf("delete one: %d", code)
	}
	if len(e.trashItems()) != 1 {
		t.Fatal("one entry should be left")
	}
	if code := e.call("anna", "POST", "/trash/empty", nil, &j); code != http.StatusAccepted || e.wait(j.ID).State != jobs.Done {
		t.Fatalf("empty: %d", code)
	}
	if len(e.trashItems()) != 0 {
		t.Fatal("trash not empty")
	}
	for _, share := range []string{"fotos", "backups"} {
		if entries, _ := os.ReadDir(e.real("/mnt/user/" + share + "/" + trash.DirName)); len(entries) != 0 {
			t.Fatalf("bin of %s still holds %d entries", share, len(entries))
		}
	}
}

func TestDeleteRemovesForGood(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "a", old)
	e.write("/mnt/user/fotos/Urlaub/2025/b.jpg", "b", old)
	j := e.run("delete", trash.PathsParams{Paths: []string{"/mnt/user/fotos/Urlaub"}})
	if !e.gone("/mnt/user/fotos/Urlaub") || j.DoneFiles != 4 || j.Undo != "" {
		t.Fatalf("delete: gone %v, %d files, undo %q", e.gone("/mnt/user/fotos/Urlaub"), j.DoneFiles, j.Undo)
	}
}

func TestWithoutRightsNothingStarts(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.jpg", "a", old)
	for kind, params := range map[string]any{
		"trash":  trash.PathsParams{Paths: []string{"/mnt/user/fotos/a.jpg"}},
		"delete": trash.PathsParams{Paths: []string{"/mnt/user/fotos/a.jpg"}},
		"move":   fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.jpg"}, Dest: "/mnt/user/backups"},
		"copy":   fileops.TransferParams{Sources: []string{"/mnt/user/fotos/a.jpg"}, Dest: "/mnt/user/backups"},
	} {
		if code := e.call("gast", "POST", "/jobs", map[string]any{"kind": kind, "params": params}, nil); code != http.StatusForbidden {
			t.Errorf("%s as guest: %d", kind, code)
		}
	}
	if code := e.call("gast", "POST", "/fs/mkdir", map[string]string{"path": "/mnt/user/fotos/neu"}, nil); code != http.StatusForbidden {
		t.Errorf("mkdir as guest: %d", code)
	}
	if code := e.call("anna", "POST", "/jobs", map[string]any{"kind": "trash", "params": trash.PathsParams{Paths: []string{"/mnt/user/fotos"}}}, nil); code != http.StatusBadRequest {
		t.Errorf("a whole share went to the trash: %d", code)
	}
}

func TestMkdirTouchRenameAndTheirUndo(t *testing.T) {
	e := newEnv(t)
	var ch fileops.Changed
	if code := e.call("anna", "POST", "/fs/mkdir", map[string]string{"path": "/mnt/user/fotos/Neu"}, &ch); code != http.StatusCreated || ch.Undo == "" {
		t.Fatalf("mkdir: %d %+v", code, ch)
	}
	if code := e.call("anna", "POST", "/fs/mkdir", map[string]string{"path": "/mnt/user/fotos/Neu"}, nil); code != http.StatusConflict {
		t.Fatalf("second mkdir: %d", code)
	}
	mkdirUndo := ch.Undo
	if code := e.call("anna", "POST", "/fs/touch", map[string]string{"path": "/mnt/user/fotos/Neu/leer.txt"}, &ch); code != http.StatusCreated {
		t.Fatalf("touch: %d", code)
	}
	touchUndo := ch.Undo
	e.write("/mnt/user/fotos/a.txt", "a", old)
	e.write("/mnt/user/fotos/b.txt", "b", old)
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/a.txt", "name": "b.txt"}, nil); code != http.StatusConflict {
		t.Fatalf("rename onto an existing name: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/a.txt", "name": "../x"}, nil); code != http.StatusBadRequest {
		t.Fatalf("rename out of the folder: %d", code)
	}
	if code := e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/a.txt", "name": "c.txt"}, &ch); code != http.StatusOK || ch.Path != "/mnt/user/fotos/c.txt" {
		t.Fatalf("rename: %d %+v", code, ch)
	}

	var steps struct {
		Steps []undo.Step `json:"steps"`
	}
	e.call("anna", "GET", "/undo", nil, &steps)
	if len(steps.Steps) != 3 || steps.Steps[0].Kind != "rename" || steps.Steps[0].To != "c.txt" {
		t.Fatalf("steps: %+v", steps.Steps)
	}
	e.undo("")
	if e.read("/mnt/user/fotos/a.txt") != "a" {
		t.Fatal("rename not taken back")
	}
	if code := e.call("anna", "POST", "/undo", map[string]string{"id": mkdirUndo}, nil); code != http.StatusConflict {
		t.Fatalf("a folder that is not empty was removed: %d", code)
	}
	e.undo(touchUndo)
	e.undo(mkdirUndo)
	if !e.gone("/mnt/user/fotos/Neu") {
		t.Fatal("folder still there")
	}
	if code := e.call("anna", "POST", "/undo", nil, nil); code != http.StatusNotFound {
		t.Fatalf("nothing left, still %d", code)
	}
	if code := e.call("bert", "GET", "/undo", nil, &steps); code != http.StatusOK || len(steps.Steps) != 0 {
		t.Fatal("another user sees the steps")
	}
}

func TestPreflightReportsConflictsAndSize(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a.txt", "neu", old.Add(time.Hour))
	e.write("/mnt/user/fotos/Urlaub/x.jpg", "12345", old)
	e.write("/mnt/user/backups/a.txt", "alt", old)
	var p fileops.Preflight
	body := map[string]any{"kind": "move", "sources": []string{"/mnt/user/fotos/a.txt", "/mnt/user/fotos/Urlaub"}, "dest": "/mnt/user/backups"}
	if code := e.call("anna", "POST", "/ops/preflight", body, &p); code != http.StatusOK {
		t.Fatalf("preflight: %d", code)
	}
	if p.Count != 2 || p.Bytes != 8 || p.Files != 3 || len(p.Conflicts) != 1 || p.Conflicts[0].Key != "a.txt" || p.Conflicts[0].Newer != "source" {
		t.Fatalf("preflight: %+v", p)
	}
	if p.Big || p.ParityBusy {
		t.Fatalf("preflight: %+v", p)
	}
}

func TestPathsCannotLeaveTheSource(t *testing.T) {
	e := newEnv(t)
	outside := filepath.Join(filepath.Dir(e.root), "geheim-"+path.Base(t.Name()))
	if err := os.WriteFile(outside, []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)
	for _, p := range []string{"/mnt/user/../geheim", "/mnt/user/fotos/../../x", "/etc/passwd"} {
		code := e.call("anna", "POST", "/jobs", map[string]any{"kind": "copy", "params": fileops.TransferParams{Sources: []string{p}, Dest: "/mnt/user/backups"}}, nil)
		if code < 400 {
			t.Errorf("%s: %d", p, code)
		}
	}
}

func (e *env) colours(folder string) map[string]string {
	e.t.Helper()
	var out struct {
		Items map[string]labels.Label `json:"items"`
	}
	if code := e.call("anna", "GET", "/labels/items?deep=true&path="+folder, nil, &out); code != http.StatusOK {
		e.t.Fatalf("labels of %s: %d", folder, code)
	}
	got := map[string]string{}
	for k, l := range out.Items {
		got[k] = l.Colour
	}
	return got
}

func TestLabelsFollowRenamesMovesCopiesAndTheTrash(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/Urlaub/a.jpg", "a", old)
	e.write("/mnt/user/fotos/b.jpg", "b", old)
	body := map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub/a.jpg", "/mnt/user/fotos/b.jpg"}, "colour": "red"}
	if code := e.call("anna", "PUT", "/labels/items", body, nil); code != http.StatusOK {
		t.Fatalf("set labels: %d", code)
	}
	e.call("anna", "POST", "/fs/rename", map[string]string{"path": "/mnt/user/fotos/Urlaub", "name": "Sommer"}, nil)
	e.run("copy", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/Sommer"}, Dest: "/mnt/user/backups"})
	e.run("move", fileops.TransferParams{Sources: []string{"/mnt/user/fotos/b.jpg"}, Dest: "/mnt/user/backups"})
	if got := e.colours("/mnt/user/backups"); got["Sommer/a.jpg"] != "red" || got["b.jpg"] != "red" {
		t.Fatalf("labels in backups: %v", got)
	}
	if got := e.colours("/mnt/user/fotos"); got["Sommer/a.jpg"] != "red" || len(got) != 1 {
		t.Fatalf("labels in fotos: %v", got)
	}
	j := e.run("trash", trash.PathsParams{Paths: []string{"/mnt/user/backups/b.jpg"}})
	e.undo(j.Undo)
	if got := e.colours("/mnt/user/backups"); got["b.jpg"] != "red" {
		t.Fatalf("label lost on the way through the trash: %v", got)
	}
	e.run("delete", trash.PathsParams{Paths: []string{"/mnt/user/backups/b.jpg"}})
	if got := e.colours("/mnt/user/backups"); got["b.jpg"] != "" {
		t.Fatalf("label outlived its file: %v", got)
	}
}
