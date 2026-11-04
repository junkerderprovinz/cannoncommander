package shares_test

import (
	"io"
	"net/http"
	"testing"
)

func FuzzVisitorsStayInsideTheLink(f *testing.F) {
	for _, p := range []string{"/", "/strand.jpg", "/2025/meer.jpg", "/../privat/tagebuch.txt", "/.geheim", "..", "/2025/../.geheim",
		`\..\privat\tagebuch.txt`, `/2025\..\..\privat\tagebuch.txt`, "/2025/..%2f..%2fprivat", "//privat/tagebuch.txt", "/strand.jpg::/x", "/\x00"} {
		f.Add(p)
	}
	e := newEnv(f)
	sh := e.create("anna", map[string]any{"paths": []string{"/mnt/user/fotos/Urlaub"}})
	f.Fuzz(func(t *testing.T, p string) {
		resp, err := http.Get(e.srv.URL + "/s/" + sh.Slug + "/raw" + q(p))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") == "application/zip" {
			return
		}
		if body := string(raw); body != "sonne" && body != "wellen" {
			t.Fatalf("%q gave %q", p, body)
		}
	})
}
