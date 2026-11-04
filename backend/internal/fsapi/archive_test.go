package fsapi

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func (f fixture) archive(t *testing.T, user string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/fs/archive-stream", bytes.NewReader(raw))
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func zipEntries(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	data := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func TestArchiveStreamZipsTheSelection(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "filme", ".cc-trash", "weg.txt"), []byte("weg"))
	put(t, filepath.Join(f.share, "filme", ".x.mkv.cc-part"), []byte("halb"))
	put(t, filepath.Join(f.share, "downloads", "liesmich.txt"), []byte("hallo"))
	rec := f.archive(t, "jdp", archiveRequest{Paths: []string{"/mnt/user/filme", "/mnt/user/downloads/liesmich.txt"}})
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename=user.zip`) {
		t.Fatalf("disposition %q", cd)
	}
	got := zipEntries(t, rec)
	if got["liesmich.txt"] != "hallo" || len(got["filme/Sintel (2010)/Sintel (2010).mkv"]) != 2048 {
		t.Fatalf("entries %v", keys(got))
	}
	for name := range got {
		if strings.Contains(name, ".cc-trash") || strings.Contains(name, ".cc-part") {
			t.Fatalf("bookkeeping in the archive: %s", name)
		}
	}
}

func TestArchiveNumbersEntriesOfTheSameName(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.share, "fotos", "a", "IMG.jpg"), []byte("eins"))
	put(t, filepath.Join(f.share, "fotos", "b", "IMG.jpg"), []byte("zwei"))
	got := zipEntries(t, f.archive(t, "jdp", archiveRequest{Paths: []string{"/mnt/user/fotos/a/IMG.jpg", "/mnt/user/fotos/b/IMG.jpg"}, Name: "bilder.zip"}))
	if got["IMG.jpg"] != "eins" || got["IMG (2).jpg"] != "zwei" {
		t.Fatalf("entries %v", keys(got))
	}
}

func TestArchiveStreamTakesAFormAndWritesTarGz(t *testing.T) {
	f := newFixture(t)
	form := url.Values{"paths": {"/mnt/user/fotos"}, "format": {"tgz"}}
	req := httptest.NewRequest(http.MethodPost, "/fs/archive-stream", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User", "jdp")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "fotos.tar.gz") {
		t.Fatalf("status %d, %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	if !slices.Contains(names, "fotos/2025/IMG_0001.jpg") || !slices.Contains(names, "fotos/2025/") {
		t.Fatalf("names %v", names)
	}
}

func TestArchiveStreamChecksEveryPath(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		user  string
		paths []string
		want  int
	}{
		{"anna", []string{"/mnt/user/filme"}, http.StatusForbidden},
		{"gast", []string{"/mnt/user/fotos"}, http.StatusForbidden},
		{"jdp", []string{"/mnt"}, http.StatusForbidden},
		{"jdp", []string{"/mnt/user/../boot"}, http.StatusBadRequest},
		{"jdp", nil, http.StatusBadRequest},
		{"jdp", []string{"/mnt/user/gibt-es-nicht"}, http.StatusNotFound},
	}
	for _, c := range cases {
		if rec := f.archive(t, c.user, archiveRequest{Paths: c.paths}); rec.Code != c.want {
			t.Errorf("%s %v: %d, want %d", c.user, c.paths, rec.Code, c.want)
		}
	}
}

func TestArchiveLeavesOutLinksThatLeaveTheBound(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Join(f.share, "fotos", "links")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(f.share), "..", "boot", "config", "go")
	if err := os.Symlink(outside, filepath.Join(dir, "raus")); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	if err := os.Symlink(filepath.Join(f.share, "fotos", "2025", "IMG_0001.jpg"), filepath.Join(dir, "drin.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.share, "fotos"), filepath.Join(dir, "schleife")); err != nil {
		t.Skip("no linked folders here:", err)
	}
	got := zipEntries(t, f.archive(t, "jdp", archiveRequest{Paths: []string{"/mnt/user/fotos/links"}}))
	if _, ok := got["links/drin.jpg"]; !ok {
		t.Fatalf("the link inside is missing: %v", keys(got))
	}
	for name := range got {
		if strings.HasPrefix(name, "links/raus") || strings.HasPrefix(name, "links/schleife/") {
			t.Fatalf("followed a link it should not: %s", name)
		}
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
