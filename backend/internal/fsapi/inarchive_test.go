package fsapi

import (
	"archive/zip"
	"encoding/base64"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeZip(t *testing.T, p string, files map[string]string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestAnArchiveOpensLikeAFolder(t *testing.T) {
	f := newFixture(t)
	writeZip(t, filepath.Join(f.share, "fotos", "urlaub.zip"), map[string]string{
		"Strand/IMG_0101.jpg": "sonne",
		"Strand/notiz.txt":    "Sand und Meer\n",
		"liesmich.txt":        "hallo",
	})

	top := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos/urlaub.zip::/"))
	if got := names(top.Items); !slices.Equal(got, []string{"Strand", "liesmich.txt"}) {
		t.Fatalf("top of the archive lists %v", got)
	}
	if !top.Items[0].Dir || top.Items[1].Size != 5 {
		t.Errorf("items are %+v", top.Items)
	}
	inner := decode[Listing](t, f.get(t, "jdp", "/fs/list", "/mnt/user/fotos/urlaub.zip::/Strand"))
	if got := names(inner.Items); !slices.Equal(got, []string{"IMG_0101.jpg", "notiz.txt"}) {
		t.Fatalf("the folder inside lists %v", got)
	}

	tree := decode[Tree](t, f.get(t, "jdp", "/fs/tree", "/mnt/user/fotos/urlaub.zip::/"))
	if len(tree.Items) != 1 || tree.Items[0].Path != "/mnt/user/fotos/urlaub.zip::/Strand" {
		t.Errorf("tree is %+v", tree.Items)
	}

	st := decode[Stat](t, f.get(t, "jdp", "/fs/stat", "/mnt/user/fotos/urlaub.zip::/Strand/notiz.txt"))
	if st.Size != int64(len("Sand und Meer\n")) || st.Dir {
		t.Errorf("stat is %+v", st)
	}

	raw := f.get(t, "jdp", "/fs/raw", "/mnt/user/fotos/urlaub.zip::/Strand/IMG_0101.jpg")
	if raw.Code != http.StatusOK || raw.Body.String() != "sonne" || raw.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("raw answered %d %q %s", raw.Code, raw.Body.String(), raw.Header().Get("Content-Type"))
	}

	text := decode[Text](t, f.get(t, "jdp", "/fs/text", "/mnt/user/fotos/urlaub.zip::/Strand/notiz.txt"))
	if text.Text != "Sand und Meer\n" || text.EOL != "lf" {
		t.Errorf("text is %+v", text)
	}

	hex := decode[Hex](t, f.get(t, "jdp", "/fs/hex", "/mnt/user/fotos/urlaub.zip::/liesmich.txt", "offset", "1", "length", "3"))
	if string(hex.Data) != "all" {
		t.Errorf("hex gave %q", base64.StdEncoding.EncodeToString(hex.Data))
	}

	if rec := f.get(t, "jdp", "/fs/raw", "/mnt/user/fotos/urlaub.zip::/fehlt.txt"); rec.Code != http.StatusNotFound {
		t.Errorf("a missing entry answered %d", rec.Code)
	}
	if rec := f.get(t, "jdp", "/fs/list", "/mnt/user/fotos/IMG.jpg::/"); rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Errorf("a path into a file that is no archive answered %d", rec.Code)
	}
	if rec := f.get(t, "anna", "/fs/list", "/mnt/user/filme/x.zip::/"); rec.Code != http.StatusForbidden {
		t.Errorf("an archive outside the scope answered %d", rec.Code)
	}
}

func TestRawFromAnArchiveSandboxesScriptableXMLTypes(t *testing.T) {
	f := newFixture(t)
	if err := mime.AddExtensionType(".rss", "application/rss+xml"); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(f.share, "fotos", "feeds.zip"), map[string]string{"feed.rss": "<rss/>", "bild.png": "sonne"})
	if csp := f.get(t, "jdp", "/fs/raw", "/mnt/user/fotos/feeds.zip::/feed.rss").Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("feed csp %q", csp)
	}
	if csp := f.get(t, "jdp", "/fs/raw", "/mnt/user/fotos/feeds.zip::/bild.png").Header().Get("Content-Security-Policy"); csp != "" {
		t.Errorf("picture csp %q", csp)
	}
}
