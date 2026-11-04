package fsapi

import (
	"bytes"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (f fixture) request(t *testing.T, user, endpoint, p string, header http.Header, extra ...string) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{"path": {p}}
	for i := 0; i+1 < len(extra); i += 2 {
		q.Set(extra[i], extra[i+1])
	}
	req := httptest.NewRequest(http.MethodGet, endpoint+"?"+q.Encode(), nil)
	for k, v := range header {
		req.Header[k] = v
	}
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func put(t *testing.T, p string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRawServesRanges(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "downloads", "zahlen.txt"), []byte("0123456789"))
	rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/zahlen.txt")
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789" {
		t.Fatalf("whole file: %d %q", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("disposition %q", cd)
	}
	tag := rec.Header().Get("ETag")
	if tag == "" || strings.HasPrefix(tag, "W/") {
		t.Fatalf("etag %q", tag)
	}

	rec = f.request(t, "jdp", "/fs/raw", "/mnt/user/downloads/zahlen.txt", http.Header{"Range": {"bytes=2-5"}})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" || rec.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %q", rec.Code, rec.Body, rec.Header().Get("Content-Range"))
	}
	rec = f.request(t, "jdp", "/fs/raw", "/mnt/user/downloads/zahlen.txt", http.Header{"Range": {"bytes=2-5"}, "If-Range": {`"old"`}})
	if rec.Code != http.StatusOK {
		t.Errorf("range against an old version: %d", rec.Code)
	}
	rec = f.request(t, "jdp", "/fs/raw", "/mnt/user/downloads/zahlen.txt", http.Header{"If-None-Match": {tag}})
	if rec.Code != http.StatusNotModified {
		t.Errorf("unchanged: %d", rec.Code)
	}
}

func TestRawDownloadsUnderTheFileName(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "downloads", "Grüße.txt"), []byte("hallo"))
	rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/Grüße.txt", "download", "1")
	cd := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "Gr%C3%BC%C3%9Fe.txt") {
		t.Fatalf("disposition %q", cd)
	}
}

func TestRawKeepsPagesFromRunningScripts(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "downloads", "seite.html"), []byte("<script>alert(1)</script>"))
	put(t, filepath.Join(f.share, "downloads", "bild.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	for _, name := range []string{"seite.html", "bild.svg"} {
		rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/"+name)
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
			t.Errorf("%s csp %q", name, csp)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s may be sniffed", name)
		}
	}
	put(t, filepath.Join(f.share, "downloads", "setup-notes.md"), []byte("# Notizen"))
	if csp := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/setup-notes.md").Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("plain text got csp %q", csp)
	}
}

func TestRawSandboxesScriptableXMLTypes(t *testing.T) {
	f := newFixture(t)
	page := []byte(`<x xmlns:h="http://www.w3.org/1999/xhtml"><h:script>alert(1)</h:script></x>`)
	for ext, ctype := range map[string]string{
		".atom": "application/atom+xml",
		".rss":  "application/rss+xml",
		".rdf":  "application/rdf+xml",
		".mml":  "application/mathml+xml",
	} {
		if err := mime.AddExtensionType(ext, ctype); err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(f.share, "downloads", "feed"+ext), page)
		rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/feed"+ext)
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
			t.Errorf("%s csp %q", ext, csp)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s may be sniffed", ext)
		}
	}
}

func TestRawLeavesWhatTheViewersShowInlineUnsandboxed(t *testing.T) {
	f := newFixture(t)
	for ext, ctype := range map[string]string{
		".png": "image/png", ".jpg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif",
		".bmp": "image/bmp", ".ico": "image/x-icon", ".mp4": "video/mp4", ".mkv": "video/x-matroska", ".mp3": "audio/mpeg",
		".flac": "audio/flac", ".pdf": "application/pdf", ".txt": "text/plain; charset=utf-8",
	} {
		if err := mime.AddExtensionType(ext, ctype); err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(f.share, "downloads", "datei"+ext), []byte("0123456789"))
		rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/datei"+ext)
		if csp := rec.Header().Get("Content-Security-Policy"); csp != "" {
			t.Errorf("%s csp %q", ext, csp)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
			t.Errorf("%s disposition %q", ext, cd)
		}
	}
	rec := f.request(t, "jdp", "/fs/raw", "/mnt/user/downloads/datei.mp4", http.Header{"Range": {"bytes=2-5"}})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" {
		t.Errorf("a video cannot seek: %d %q", rec.Code, rec.Body)
	}
}

func TestRawSandboxesATypeItDoesNotKnow(t *testing.T) {
	f := newFixture(t)
	if err := mime.AddExtensionType(".frisch", "application/x-frisch"); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.share, "downloads", "neu.frisch"), []byte("x"))
	if csp := f.get(t, "jdp", "/fs/raw", "/mnt/user/downloads/neu.frisch").Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("csp %q", csp)
	}
}

func TestContentNeedsTheDownloadRight(t *testing.T) {
	f := newFixture(t)
	for _, endpoint := range []string{"/fs/raw", "/fs/text", "/fs/hex"} {
		if rec := f.get(t, "gast", endpoint, "/mnt/user/downloads/setup-notes.md"); rec.Code != http.StatusForbidden {
			t.Errorf("%s for a viewer: %d", endpoint, rec.Code)
		}
	}
	if rec := f.get(t, "gast", "/fs/list", "/mnt/user/downloads"); rec.Code != http.StatusOK {
		t.Errorf("list for a viewer: %d", rec.Code)
	}
}

func TestContentRejectsFoldersAndMissingArchives(t *testing.T) {
	f := newFixture(t)
	cases := map[string]int{
		"/mnt/user/downloads":           http.StatusBadRequest,
		"/mnt":                          http.StatusForbidden,
		"/mnt/user/a.zip::/b.txt":       http.StatusNotFound,
		"/mnt/user/downloads/fehlt.txt": http.StatusNotFound,
		"":                              http.StatusBadRequest,
	}
	for _, endpoint := range []string{"/fs/raw", "/fs/text", "/fs/hex"} {
		for p, want := range cases {
			if rec := f.get(t, "jdp", endpoint, p); rec.Code != want {
				t.Errorf("%s %q: %d, want %d", endpoint, p, rec.Code, want)
			}
		}
	}
}

func TestTextDetectsTheEncoding(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name, encoding, text, eol string
		data                      []byte
	}{
		{"ascii.txt", "ascii", "a\nb\n", "lf", []byte("a\nb\n")},
		{"utf8.txt", "utf-8", "Grüße\r\n", "crlf", []byte("Grüße\r\n")},
		{"bom.txt", "utf-8-bom", "Grüße", "", append([]byte{0xEF, 0xBB, 0xBF}, "Grüße"...)},
		{"le.txt", "utf-16le", "Hé", "", []byte{0xFF, 0xFE, 'H', 0, 0xE9, 0}},
		{"be.txt", "utf-16be", "Hé", "", []byte{0xFE, 0xFF, 0, 'H', 0, 0xE9}},
		{"ansi.txt", "windows-1252", "Grüße €", "", []byte{'G', 'r', 0xFC, 0xDF, 'e', ' ', 0x80}},
		{"gemischt.txt", "ascii", "a\nb\r\nc", "mixed", []byte("a\nb\r\nc")},
	}
	for _, c := range cases {
		put(t, filepath.Join(f.share, "texte", c.name), c.data)
		got := decode[Text](t, f.get(t, "jdp", "/fs/text", "/mnt/user/texte/"+c.name))
		if got.Encoding != c.encoding || got.Text != c.text || got.EOL != c.eol || got.Truncated || got.Size != int64(len(c.data)) {
			t.Errorf("%s = %+v", c.name, got)
		}
	}
}

func TestTextStopsAtMaxWithoutBreakingACharacter(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "texte", "lang.txt"), []byte("abü"))
	got := decode[Text](t, f.get(t, "jdp", "/fs/text", "/mnt/user/texte/lang.txt", "max", "3"))
	if got.Text != "ab" || !got.Truncated || got.Encoding != "ascii" {
		t.Fatalf("cut text = %+v", got)
	}
	if rec := f.get(t, "jdp", "/fs/text", "/mnt/user/texte/lang.txt", "max", "0"); rec.Code != http.StatusBadRequest {
		t.Errorf("max 0: %d", rec.Code)
	}
}

func TestTextRefusesBinaryFiles(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "downloads", "programm.bin"), []byte{0x7F, 'E', 'L', 'F', 0, 0, 1})
	if rec := f.get(t, "jdp", "/fs/text", "/mnt/user/downloads/programm.bin"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binary: %d", rec.Code)
	}
}

func TestHexReturnsTheAskedBytes(t *testing.T) {
	f := newFixture(t)
	data := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 3)
	put(t, filepath.Join(f.share, "downloads", "daten.bin"), data)
	got := decode[Hex](t, f.get(t, "jdp", "/fs/hex", "/mnt/user/downloads/daten.bin", "offset", "2", "length", "4"))
	if got.Size != 12 || got.Offset != 2 || !bytes.Equal(got.Data, []byte{0xBE, 0xEF, 0xDE, 0xAD}) {
		t.Fatalf("hex = %+v", got)
	}
	got = decode[Hex](t, f.get(t, "jdp", "/fs/hex", "/mnt/user/downloads/daten.bin", "offset", "10"))
	if !bytes.Equal(got.Data, []byte{0xBE, 0xEF}) {
		t.Errorf("tail = %x", got.Data)
	}
	got = decode[Hex](t, f.get(t, "jdp", "/fs/hex", "/mnt/user/downloads/daten.bin", "offset", "99"))
	if len(got.Data) != 0 {
		t.Errorf("past the end = %x", got.Data)
	}
	if rec := f.get(t, "jdp", "/fs/hex", "/mnt/user/downloads/daten.bin", "length", "70000"); rec.Code != http.StatusBadRequest {
		t.Errorf("too long: %d", rec.Code)
	}
}
