package fsapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func (f fixture) save(t *testing.T, user, p string, body any, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/fs/file?"+url.Values{"path": {p}}.Encode(), strings.NewReader(string(raw)))
	req.Header.Set("X-User", user)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func TestEditorSavesOnlyOverWhatItRead(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "downloads", "notes.md"), []byte("# Hallo\n"))
	got := decode[EditFile](t, f.get(t, "jdp", "/fs/file", "/mnt/user/downloads/notes.md"))
	if got.Text != "# Hallo\n" || got.Encoding != "ascii" || got.EOL != "lf" || got.ETag == "" || !got.Writable {
		t.Fatalf("read %+v", got)
	}

	rec := f.save(t, "jdp", "/mnt/user/downloads/notes.md", map[string]string{"text": "# Hallo Welt\n"}, "If-Match", got.ETag)
	saved := decode[SavedFile](t, rec)
	if saved.ETag == got.ETag || rec.Header().Get("ETag") != saved.ETag || saved.Size != 13 {
		t.Fatalf("saved %+v", saved)
	}
	if rec := f.save(t, "jdp", "/mnt/user/downloads/notes.md", map[string]string{"text": "alt"}, "If-Match", got.ETag); rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("a save over a changed file: %d", rec.Code)
	}
	if b, _ := os.ReadFile(filepath.Join(f.share, "downloads", "notes.md")); string(b) != "# Hallo Welt\n" {
		t.Fatalf("content %q", b)
	}
	if rec := f.request(t, "jdp", "/fs/file", "/mnt/user/downloads/notes.md", http.Header{"If-None-Match": {saved.ETag}}); rec.Code != http.StatusNotModified {
		t.Fatalf("unchanged read: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(f.share, "downloads", ".notes.md.cc-part")); !os.IsNotExist(err) {
		t.Fatal("the part file stayed")
	}
}

func TestEditorWritesBackTheEncodingAndLineEndings(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.share, "downloads", "alt.txt")
	put(t, p, []byte("Gr\xfc\xdfe\r\nZeile\r\n"))
	got := decode[EditFile](t, f.get(t, "jdp", "/fs/file", "/mnt/user/downloads/alt.txt"))
	if got.Encoding != "windows-1252" || got.EOL != "crlf" || got.Text != "Grüße\r\nZeile\r\n" {
		t.Fatalf("read %+v", got)
	}
	f.save(t, "jdp", "/mnt/user/downloads/alt.txt", map[string]string{"text": "Grüße\nneu\n", "eol": "crlf"})
	if b, _ := os.ReadFile(p); string(b) != "Gr\xfc\xdfe\r\nneu\r\n" {
		t.Fatalf("written %q", b)
	}
	if rec := f.save(t, "jdp", "/mnt/user/downloads/alt.txt", map[string]string{"text": "✓"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a character Windows-1252 lacks: %d", rec.Code)
	}

	put(t, filepath.Join(f.share, "downloads", "wide.txt"), []byte{0xFF, 0xFE, 'a', 0})
	f.save(t, "jdp", "/mnt/user/downloads/wide.txt", map[string]string{"text": "ab"})
	if b, _ := os.ReadFile(filepath.Join(f.share, "downloads", "wide.txt")); string(b) != "\xff\xfea\x00b\x00" {
		t.Fatalf("utf-16 written as %q", b)
	}
}

func TestEditorCreatesNewFiles(t *testing.T) {
	f := newFixture(t)
	rec := f.save(t, "jdp", "/mnt/user/downloads/neu.txt", map[string]string{"text": "hallo"}, "If-None-Match", "*")
	if rec.Code != http.StatusCreated {
		t.Fatalf("new file: %d %s", rec.Code, rec.Body)
	}
	if rec := f.save(t, "jdp", "/mnt/user/downloads/neu.txt", map[string]string{"text": "x"}, "If-None-Match", "*"); rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("new file over an existing one: %d", rec.Code)
	}
	if rec := f.save(t, "jdp", "/mnt/user/downloads/gibt-es-nicht.txt", map[string]string{"text": "x"}, "If-Match", `"abc"`); rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("If-Match on a missing file: %d", rec.Code)
	}
	if rec := f.save(t, "jdp", "/mnt/user/nirgends/neu.txt", map[string]string{"text": "x"}); rec.Code != http.StatusConflict {
		t.Fatalf("new file in a missing folder: %d", rec.Code)
	}
}

func TestEditorNeedsTheRights(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "fotos", "liste.txt"), []byte("eins"))
	if rec := f.get(t, "gast", "/fs/file", "/mnt/user/fotos/liste.txt"); rec.Code != http.StatusForbidden {
		t.Fatalf("read without the download right: %d", rec.Code)
	}
	if rec := f.save(t, "gast", "/mnt/user/fotos/liste.txt", map[string]string{"text": "zwei"}); rec.Code != http.StatusForbidden {
		t.Fatalf("save without the modify right: %d", rec.Code)
	}
	if rec := f.save(t, "anna", "/mnt/user/downloads/setup-notes.md", map[string]string{"text": "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("save outside the scope: %d", rec.Code)
	}
	put(t, filepath.Join(f.share, "..", "..", "boot", "config", "go"), []byte("#!/bin/bash\n"))
	got := decode[EditFile](t, f.get(t, "jdp", "/fs/file", "/boot/config/go"))
	if got.Writable {
		t.Fatal("a file on a read only source is writable")
	}
	if rec := f.save(t, "jdp", "/boot/config/go", map[string]string{"text": "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("save on a read only source: %d", rec.Code)
	}
}

func TestEditorRefusesBinaryFiles(t *testing.T) {
	f := newFixture(t)
	if rec := f.get(t, "jdp", "/fs/file", "/mnt/user/fotos/2025/IMG_0001.jpg"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binary file: %d", rec.Code)
	}
	if rec := f.get(t, "jdp", "/fs/file", "/mnt/user/fotos"); rec.Code != http.StatusBadRequest {
		t.Fatalf("folder: %d", rec.Code)
	}
}

func TestEditorKeepsModeAndHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modes and link counts are a unix matter")
	}
	f := newFixture(t)
	p := filepath.Join(f.share, "downloads", "conf.ini")
	put(t, p, []byte("a=1"))
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	f.save(t, "jdp", "/mnt/user/downloads/conf.ini", map[string]string{"text": "a=2"})
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", info.Mode())
	}
	other := filepath.Join(f.share, "fotos", "conf-link.ini")
	if err := os.Link(p, other); err != nil {
		t.Fatal(err)
	}
	f.save(t, "jdp", "/mnt/user/downloads/conf.ini", map[string]string{"text": "a=3"})
	if b, _ := os.ReadFile(other); string(b) != "a=3" {
		t.Fatalf("the hard link sees %q", b)
	}
}

func TestEditorConvertsToTheEncodingAsked(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.share, "downloads", "preis.txt")
	put(t, p, []byte("5 €\n"))
	f.save(t, "jdp", "/mnt/user/downloads/preis.txt", map[string]string{"text": "5 €\n", "encoding": "iso-8859-15", "eol": "crlf"})
	if b, _ := os.ReadFile(p); string(b) != "5 \xa4\r\n" {
		t.Fatalf("written %q", b)
	}
	got := decode[EditFile](t, f.get(t, "jdp", "/fs/file", "/mnt/user/downloads/preis.txt", "encoding", "iso-8859-15"))
	if got.Encoding != "iso-8859-15" || got.Text != "5 €\r\n" {
		t.Fatalf("read back %+v", got)
	}
	rec := f.save(t, "jdp", "/mnt/user/downloads/preis.txt", map[string]string{"text": "✓", "encoding": "iso-8859-1"})
	var problem struct {
		Code   string         `json:"code"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil || rec.Code != http.StatusUnprocessableEntity || problem.Code != "notEncodable" || problem.Params["encoding"] != "ISO-8859-1" {
		t.Fatalf("a character ISO-8859-1 lacks: %d %s", rec.Code, rec.Body)
	}
	if rec := f.save(t, "jdp", "/mnt/user/downloads/preis.txt", map[string]string{"text": "x", "encoding": "ebcdic"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown encoding: %d", rec.Code)
	}
}
