package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/desktop/modules"
	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// upstream serves files the way the pinned hosts do.
func upstream(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAssembleKeepsWhatTheSourcesNameAndRecordsTheVersions(t *testing.T) {
	archive := zipOf(t, map[string]string{
		"tool-1.0/bin/tool":    "the program",
		"tool-1.0/bin/helper":  "a helper it needs",
		"tool-1.0/doc/manual":  "left out",
		"tool-1.0/LICENSE.txt": "the licence",
	})
	notice := []byte("a licence of its own")
	srv := upstream(t, map[string][]byte{"tool-1.0.zip": archive, "NOTICE": notice})
	src := modules.Sources{"linux/amd64": {"tool": {
		Version: "1.0", Path: "bin", Env: map[string]string{"CC_TOOL": "bin/tool"},
		Sources: []modules.Source{
			{URL: srv.URL + "/tool-1.0.zip", SHA256: sum(archive), Keep: map[string]string{"*/bin/*": "bin", "*/LICENSE.txt": "."}},
			{URL: srv.URL + "/NOTICE", SHA256: sum(notice), Keep: map[string]string{"NOTICE": "."}},
		},
	}}}
	out := filepath.Join(t.TempDir(), "modules")
	a := &assembler{client: srv.Client()}
	if err := a.assemble(context.Background(), src, "linux/amd64", "v1.4.0", out); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"tool/bin/tool":    "the program",
		"tool/bin/helper":  "a helper it needs",
		"tool/LICENSE.txt": "the licence",
		"tool/NOTICE":      "a licence of its own",
	} {
		if got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name))); err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "tool", "doc")); err == nil {
		t.Error("the manual came along although nothing keeps it")
	}
	var rec modules.Record
	data, err := os.ReadFile(filepath.Join(out, modules.RecordFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	tool := rec.Modules["tool"]
	if rec.Release != "v1.4.0" || rec.Platform != "linux/amd64" || tool.Version != "1.0" || tool.Path != "bin" || tool.Env["CC_TOOL"] != "bin/tool" {
		t.Errorf("record %+v", rec)
	}
	if len(tool.Sources) != 0 {
		t.Error("the record carries the sources")
	}
}

func TestAssembleRefusesADownloadThatIsNotThePinnedOne(t *testing.T) {
	archive := zipOf(t, map[string]string{"tool": "changed upstream"})
	srv := upstream(t, map[string][]byte{"tool.zip": archive})
	src := modules.Sources{"linux/amd64": {"tool": {Version: "1", Path: ".", Sources: []modules.Source{
		{URL: srv.URL + "/tool.zip", SHA256: sum([]byte("what was pinned")), Keep: map[string]string{"tool": "."}},
	}}}}
	a := &assembler{client: srv.Client()}
	err := a.assemble(context.Background(), src, "linux/amd64", "v1.4.0", filepath.Join(t.TempDir(), "modules"))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("got %v, want a checksum error", err)
	}
}

func TestAssembleFailsWhenAKeptFileIsMissing(t *testing.T) {
	archive := zipOf(t, map[string]string{"tool-2.0/bin/tool": "renamed folder"})
	srv := upstream(t, map[string][]byte{"tool.zip": archive})
	src := modules.Sources{"linux/amd64": {"tool": {Version: "2", Path: ".", Sources: []modules.Source{
		{URL: srv.URL + "/tool.zip", SHA256: sum(archive), Keep: map[string]string{"*/bin/tool": ".", "*/COPYING": "."}},
	}}}}
	a := &assembler{client: srv.Client()}
	err := a.assemble(context.Background(), src, "linux/amd64", "v1.4.0", filepath.Join(t.TempDir(), "modules"))
	if err == nil || !strings.Contains(err.Error(), "COPYING") {
		t.Fatalf("got %v, want the missing licence named", err)
	}
}

func TestTheZipUnpacksToTheSameFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "modules")
	if err := os.MkdirAll(filepath.Join(dir, "tool", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "tool", "bin", "tool"), []byte("program"), 0o755)
	os.WriteFile(filepath.Join(dir, modules.RecordFile), []byte("{}"), 0o644)
	name := filepath.Join(t.TempDir(), "modules.zip")
	if err := writeZip(dir, name); err != nil {
		t.Fatal(err)
	}
	back := t.TempDir()
	if err := update.Unzip(name, back); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(back, "tool", "bin", "tool")); string(got) != "program" {
		t.Errorf("program %q", got)
	}
	if _, err := os.Stat(filepath.Join(back, modules.RecordFile)); err != nil {
		t.Error(err)
	}
}
