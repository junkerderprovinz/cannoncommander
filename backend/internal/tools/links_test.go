package tools

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/archives"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// linkTo makes a link at the API path at that leads to the API path target.
func (e *env) linkTo(at, target string) {
	e.t.Helper()
	linksOnly(e.t)
	if err := os.Symlink(e.real(target), e.real(at)); err != nil {
		e.t.Fatal(err)
	}
}

func TestChecksumThroughALinkedFolderLeavesOutWhatIsHidden(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/album/frei.txt", "frei")
	e.write("/mnt/user/fotos/album/geheim/x.txt", "geheim")
	e.rules.set("/mnt/user/fotos/album/geheim", ccfs.Hidden)
	e.linkTo("/mnt/user/downloads/tor", "/mnt/user/fotos")

	j := e.run(KindChecksum, map[string]any{"sources": []string{"/mnt/user/downloads/tor/album"}, "algo": "sha256"})
	for _, s := range result[struct {
		Files []FileSum `json:"files"`
	}](t, j).Files {
		if filepath.Base(s.Path) == "x.txt" {
			t.Errorf("hashed %s", s.Path)
		}
	}

	e.run(KindPack, map[string]any{"sources": []string{"/mnt/user/downloads/tor/album"}, "dest": "/mnt/user/backups", "name": "alle", "format": "zip"})
	ix, err := archives.Load(e.real("/mnt/user/backups/alle.zip"), archives.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Stat("/album/geheim/x.txt"); ok {
		t.Error("the hidden file was packed")
	}
}

func TestCompareThroughALinkGoesByTheScopeAndRulesWhereItLeads(t *testing.T) {
	e := newEnv(t)
	e.scopes["carl"] = "/fotos"
	e.write("/mnt/user/backups/lohn.txt", "x")
	e.write("/mnt/user/fotos/zwei/a.txt", "x")
	e.linkTo("/mnt/user/fotos/oben", "/mnt/user")
	var out struct {
		Items []Compared `json:"items"`
	}
	code := e.call("carl", "POST", "/tools/compare", CompareRequest{Left: "/mnt/user/fotos/oben", Right: "/mnt/user/fotos/zwei", Top: true}, &out)
	if code != http.StatusOK {
		t.Fatalf("compare: %d", code)
	}
	for _, it := range out.Items {
		if it.Left != nil && (it.Name == "backups" || it.Name == "downloads") {
			t.Errorf("shows %s from outside the scope", it.Name)
		}
	}
}

func TestSyncDoesNotReplaceAReadOnlyFileThroughALink(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/fest.txt", "alt")
	e.write("/mnt/user/downloads/fest.txt", "neu")
	e.rules.set("/mnt/user/fotos/fest.txt", ccfs.ReadOnly)
	e.linkTo("/mnt/user/backups/tor", "/mnt/user/fotos")
	code := e.startAnswer(KindSync, SyncParams{Left: "/mnt/user/downloads", Right: "/mnt/user/backups/tor",
		Steps: []SyncStep{{Rel: "fest.txt", Act: "replace", To: "R"}}})
	if code != http.StatusForbidden {
		t.Errorf("%d", code)
	}
}

func TestUnpackIntoItsOwnFolderDoesNotFollowALinkOfThatName(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/frei.txt", "frei")
	e.rules.set("/mnt/user/fotos", ccfs.ReadOnly)
	e.linkTo("/mnt/user/downloads/a", "/mnt/user/fotos")
	tarWith(t, e.real("/mnt/user/backups/a.tar"), nil, map[string]string{"neu.txt": "neu"})
	e.finish(KindUnpack, map[string]any{"archives": []string{"/mnt/user/backups/a.tar"}, "dest": "/mnt/user/downloads", "own": true})
	if e.exists("/mnt/user/fotos/neu.txt") {
		t.Error("the member landed in the read only folder")
	}
}

func TestSplitDoesNotWriteItsSumThroughALink(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/fest.txt", "alt")
	e.write("/mnt/user/downloads/gross.bin", strings.Repeat("0123456789abcdef", 1<<16+100))
	e.rules.set("/mnt/user/fotos/fest.txt", ccfs.ReadOnly)
	e.linkTo("/mnt/user/backups/gross.bin.sha256", "/mnt/user/fotos/fest.txt")
	e.finish(KindSplit, map[string]any{"path": "/mnt/user/downloads/gross.bin", "dest": "/mnt/user/backups", "partSize": 1 << 20, "sum": true})
	if got := e.read("/mnt/user/fotos/fest.txt"); got != "alt" {
		t.Errorf("the read only file changed: %q", got)
	}
}

func TestCombineAnswersForRulesOnEveryPart(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/downloads/g.bin", strings.Repeat("0123456789abcdef", 1<<16+100))
	e.run(KindSplit, map[string]any{"path": "/mnt/user/downloads/g.bin", "dest": "/mnt/user/fotos", "partSize": 1 << 20})
	e.rules.set("/mnt/user/fotos/g.bin.002", ccfs.Hidden)
	e.finish(KindCombine, map[string]any{"path": "/mnt/user/fotos/g.bin.001", "name": "ganz.bin", "dest": "/mnt/user/backups"})
	if e.exists("/mnt/user/backups/ganz.bin") {
		t.Error("the result holds a part that is hidden")
	}
}

func TestAFetchedFileCannotNameItselfAfterTheBin(t *testing.T) {
	e := newEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename=".cc-trash"`)
		w.Write([]byte("x"))
	}))
	defer server.Close()
	saved := fetchClient.Transport
	fetchClient.Transport = &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}
	defer func() { fetchClient.Transport = saved }()
	e.finish(KindFetch, FetchParams{URL: "http://example.org/datei", Dest: "/mnt/user/fotos"})
	if _, err := os.Lstat(e.real("/mnt/user/fotos/.cc-trash")); err == nil {
		t.Error("the server chose the name of the bin")
	}
}

func TestUndoOfABatchRenameAnswersForRulesInsideTheFolder(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/fotos/a/x.txt", "x")
	var out BatchResult
	if code := e.call("anna", "POST", "/fs/rename/batch", BatchRequest{Paths: []string{"/mnt/user/fotos/a"}, Names: []string{"b"}}, &out); code != http.StatusOK {
		t.Fatalf("rename: %d", code)
	}
	e.rules.set("/mnt/user/fotos/b/x.txt", ccfs.ReadOnly)
	e.call("anna", "POST", "/undo", map[string]string{"id": out.Undo}, nil)
	if !e.exists("/mnt/user/fotos/b/x.txt") {
		t.Error("undo took the file out from under its rule")
	}
}

func TestContentHashesDoNotReadThroughALink(t *testing.T) {
	linksOnly(t)
	f := newFixture(t)
	outside := filepath.Join(t.TempDir(), "lohn.txt")
	if err := os.WriteFile(outside, []byte("gehalt gehalt gehalt"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.share, "docs", "tor.txt")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.share, "docs", "gleich.txt"), "gehalt gehalt gehalt", old)
	d := f.duplicates(t, DuplicatesParams{Path: "/mnt/user/docs", By: "size"})
	for _, g := range d.Groups {
		for _, c := range g.Copies {
			if filepath.Base(c.Path) == "tor.txt" {
				t.Errorf("the file behind a link got hashed: %+v", g)
			}
		}
	}
}
