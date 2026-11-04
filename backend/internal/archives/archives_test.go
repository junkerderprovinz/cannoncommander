package archives

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var fixedTime = time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC)

// makeTree builds the folder every format packs: text with umlauts, an
// empty folder, a small binary and, where the system allows, a symlink.
func makeTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	files := map[string]string{
		"docs/readme.txt":   "Hallo Welt\n",
		"docs/Grüße.txt":    "Servus aus Wien\n",
		"fotos/strand.jpg":  strings.Repeat("\xff\xd8\xff\xe0JFIF", 300),
		"fotos/2026/a.txt":  "a",
		"leer.txt":          "",
		"docs/leer/.keep":   "",
		"Tabelle (1).csv":   "a;b\n1;2\n",
		"tief/x/y/z/end.md": "# Ende\n",
	}
	for name, body := range files {
		p := filepath.Join(base, "set", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink("docs/readme.txt", filepath.Join(base, "set", "verweis")); err != nil {
			t.Fatal(err)
		}
	}
	_ = filepath.WalkDir(filepath.Join(base, "set"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type()&fs.ModeSymlink == 0 {
			_ = os.Chtimes(p, fixedTime, fixedTime)
		}
		return nil
	})
	return base
}

// listing prints an index the way the golden files keep it.
func listing(t *testing.T, ix *Index) string {
	t.Helper()
	var lines []string
	var walk func(dir string)
	walk = func(dir string) {
		kids, ok := ix.Children(dir)
		if !ok {
			t.Fatalf("no folder %q", dir)
		}
		for _, e := range kids {
			kind := "file"
			switch {
			case e.Dir:
				kind = "dir"
			case e.Symlink():
				kind = "link " + e.Link
			}
			lines = append(lines, fmt.Sprintf("%s\t%s\t%d", e.Name, kind, e.Size))
			if e.Dir {
				walk(e.Name)
			}
		}
	}
	walk("/")
	return strings.Join(lines, "\n") + "\n"
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

func packFormats(t *testing.T) []Format {
	var out []Format
	for _, f := range PackFormats {
		if CanPack(f) {
			out = append(out, f)
		} else {
			t.Logf("%s cannot be written here", f)
		}
	}
	return out
}

func TestEveryFormatPacksAndReadsTheSameTree(t *testing.T) {
	base := makeTree(t)
	for _, f := range packFormats(t) {
		t.Run(string(f), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "set."+string(f))
			if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: f, Level: 6}); err != nil {
				t.Fatalf("pack: %v", err)
			}
			ix, err := Load(out, Options{})
			if err != nil {
				t.Fatalf("index: %v", err)
			}
			name := "tree.txt"
			if runtime.GOOS == "windows" {
				name = "tree-nolink.txt"
			}
			golden(t, name, listing(t, ix))
		})
	}
}

func TestUnpackingGivesBackTheSameBytesAndTimes(t *testing.T) {
	base := makeTree(t)
	for _, f := range packFormats(t) {
		t.Run(string(f), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "set."+string(f))
			if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: f, Level: 3}); err != nil {
				t.Fatal(err)
			}
			a, err := Open(out, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			dest := t.TempDir()
			res, err := Extract(a, dest, ExtractOptions{})
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if !slices.Equal(res.Top, []string{"set"}) {
				t.Errorf("top level is %v", res.Top)
			}
			sameTree(t, filepath.Join(base, "set"), filepath.Join(dest, "set"))
		})
	}
}

func sameTree(t *testing.T, want, got string) {
	t.Helper()
	err := filepath.WalkDir(want, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(want, p)
		q := filepath.Join(got, rel)
		info, err := os.Lstat(q)
		if err != nil {
			t.Errorf("%s is missing", rel)
			return nil
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			a, _ := os.Readlink(p)
			b, _ := os.Readlink(q)
			if a != b {
				t.Errorf("%s points to %q, want %q", rel, b, a)
			}
		case d.Type().IsRegular():
			a, _ := os.ReadFile(p)
			b, _ := os.ReadFile(q)
			if string(a) != string(b) {
				t.Errorf("%s has other content", rel)
			}
			if !info.ModTime().Equal(fixedTime) {
				t.Errorf("%s has time %v", rel, info.ModTime())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPasswordZipNeedsThePassword(t *testing.T) {
	base := makeTree(t)
	out := filepath.Join(t.TempDir(), "geheim.zip")
	if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: Zip, Level: 6, Password: "Bergkäse"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Test(out, Options{}, nil); !errors.Is(err, ErrPassword) {
		t.Errorf("without a password the test said %v", err)
	}
	if _, err := Test(out, Options{Password: "falsch"}, nil); !errors.Is(err, ErrPassword) {
		t.Errorf("with a wrong password the test said %v", err)
	}
	if _, err := Test(out, Options{Password: "Bergkäse"}, nil); err != nil {
		t.Errorf("with the password the test said %v", err)
	}
	a, err := Open(out, Options{Password: "Bergkäse"})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	dest := t.TempDir()
	if _, err := Extract(a, dest, ExtractOptions{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "set", "docs", "Grüße.txt")); string(b) != "Servus aus Wien\n" {
		t.Errorf("decrypted content is %q", b)
	}
}

func TestTheFixturesOfOtherToolsRead(t *testing.T) {
	fixtures, _ := filepath.Glob(filepath.Join("testdata", "fixtures", "*"))
	if len(fixtures) == 0 {
		t.Skip("no fixtures")
	}
	for _, p := range fixtures {
		t.Run(filepath.Base(p), func(t *testing.T) {
			ix, err := Load(p, Options{Password: fixturePassword})
			if err != nil {
				t.Fatalf("index: %v", err)
			}
			golden(t, "fixture-"+filepath.Base(p)+".txt", listing(t, ix))
			if _, err := Test(p, Options{Password: fixturePassword}, nil); err != nil {
				t.Errorf("test: %v", err)
			}
		})
	}
}

func TestADamagedArchiveNamesTheMember(t *testing.T) {
	base := makeTree(t)
	out := filepath.Join(t.TempDir(), "set.zip")
	if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: Zip, Level: 0}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	i := strings.Index(string(b), "Servus aus Wien")
	b[i] = 'X'
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Test(out, Options{}, nil)
	var d *Damage
	if !errors.As(err, &d) || d.Entry != "set/docs/Grüße.txt" {
		t.Errorf("the damage is reported as %v", err)
	}
}

func TestClashRules(t *testing.T) {
	base := makeTree(t)
	out := filepath.Join(t.TempDir(), "set.zip")
	if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: Zip, Level: 1}); err != nil {
		t.Fatal(err)
	}
	extract := func(dest, clash string) ExtractResult {
		a, err := Open(out, Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		res, err := Extract(a, dest, ExtractOptions{Clash: clash})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for _, c := range []struct {
		clash, top string
		kept       bool
	}{{ClashSkip, "", true}, {ClashBoth, "set (1)", true}, {ClashReplace, "set", false}} {
		dest := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dest, "set"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, "set", "mine.txt"), []byte("mine"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := extract(dest, c.clash)
		if got := strings.Join(res.Top, ","); got != c.top {
			t.Errorf("%s: top is %q, want %q", c.clash, got, c.top)
		}
		if _, err := os.Stat(filepath.Join(dest, "set", "mine.txt")); (err == nil) != c.kept {
			t.Errorf("%s: the existing file kept is %v", c.clash, err == nil)
		}
	}
}

func TestPrefixAndOnlyPickPartOfTheArchive(t *testing.T) {
	base := makeTree(t)
	out := filepath.Join(t.TempDir(), "set.tar.zst")
	if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: TarZst, Level: 3}); err != nil {
		t.Fatal(err)
	}
	a, err := Open(out, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	dest := t.TempDir()
	res, err := Extract(a, dest, ExtractOptions{Prefix: "set/docs", Only: []string{"readme.txt", "leer"}})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(res.Top)
	if !slices.Equal(res.Top, []string{"leer", "readme.txt"}) {
		t.Errorf("top is %v", res.Top)
	}
	if _, err := os.Stat(filepath.Join(dest, "Grüße.txt")); err == nil {
		t.Error("a name that was not picked was unpacked")
	}
}

func TestReadEntryFindsOneFile(t *testing.T) {
	base := makeTree(t)
	out := filepath.Join(t.TempDir(), "set.tar.gz")
	if err := Pack(context.Background(), out, base, []string{"set"}, PackOptions{Format: TarGz, Level: 9}); err != nil {
		t.Fatal(err)
	}
	var got string
	err := ReadEntry(out, Options{}, "/set/docs/readme.txt", func(e Entry, r io.Reader) error {
		b, err := io.ReadAll(r)
		got = string(b)
		return err
	})
	if err != nil || got != "Hallo Welt\n" {
		t.Errorf("got %q, %v", got, err)
	}
	if err := ReadEntry(out, Options{}, "/set/nope", func(Entry, io.Reader) error { return nil }); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing entry gave %v", err)
	}
}

func TestFormatNames(t *testing.T) {
	for name, want := range map[string]Format{
		"a.ZIP": Zip, "b.tar.gz": TarGz, "c.tgz": TarGz, "d.tar.zst": TarZst, "e.7z": SevenZ,
		"f.rar": Rar, "g.tar": Tar, "h.txt": "", ".zip": "", "i.tar.bz2": TarBz2, "j.txz": TarXz,
	} {
		if got := FormatOf(name); got != want {
			t.Errorf("FormatOf(%q) = %q, want %q", name, got, want)
		}
	}
	if got := StripExt("fotos.tar.zst"); got != "fotos" {
		t.Errorf("StripExt gave %q", got)
	}
}

// fixturePassword locks the encrypted fixtures. It is plain ASCII, as rar
// turns a password from the command line into bytes by the locale.
const fixturePassword = "Bergkaese"
