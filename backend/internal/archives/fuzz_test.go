package archives

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeArchive yields entries from a script, one per line: "f name",
// "d name", "l name target" or "h name target". It stands for every format,
// since all of them hand their raw names to entryFor and Extract.
type fakeArchive struct{ script string }

func (a fakeArchive) Close() error { return nil }

func (a fakeArchive) Walk(fn func(Entry, Opener) error) error {
	for _, line := range strings.Split(a.script, "\n") {
		kind, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name, target, _ := strings.Cut(rest, " ")
		var e Entry
		var err error
		switch kind {
		case "f":
			e, err = entryFor(name, 0o644, 4, time.Time{}, "")
		case "d":
			e, err = entryFor(name, fs.ModeDir|0o755, 0, time.Time{}, "")
		case "l":
			e, err = entryFor(name, fs.ModeSymlink|0o777, 0, time.Time{}, target)
		case "h":
			e, err = entryFor(name, 0o644, 0, time.Time{}, "")
			if err == nil {
				e.HardLink, err = CleanName(target)
			}
		default:
			continue
		}
		if err != nil {
			continue
		}
		if err := fn(e, func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("evil")), nil }); err != nil {
			return err
		}
	}
	return nil
}

// checkContained fails when anything appeared next to dest or a link in
// dest leads out of it.
func checkContained(t *testing.T, parent, dest string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "dest" && e.Name() != "outside" {
			t.Fatalf("%s appeared next to the target", e.Name())
		}
	}
	if left, _ := os.ReadDir(filepath.Join(parent, "outside")); len(left) != 1 {
		t.Fatalf("the folder outside was changed: %v", left)
	}
	_ = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, _ := os.Readlink(p)
		resolved := filepath.Join(filepath.Dir(p), target)
		rel, err := filepath.Rel(dest, resolved)
		if filepath.IsAbs(target) || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("%s points outside with %s", p, target)
		}
		return nil
	})
}

func sandbox(t *testing.T) (parent, dest string) {
	parent = t.TempDir()
	dest = filepath.Join(parent, "dest")
	outside := filepath.Join(parent, "outside")
	for _, d := range []string{dest, outside} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	return parent, dest
}

var slipScripts = []string{
	"f ../evil.txt",
	"f ../../evil.txt",
	"f /etc/evil.txt",
	"f ..\\evil.txt",
	"f C:\\evil.txt",
	"f c:/evil.txt",
	"f a/../../evil.txt",
	"f ./../evil.txt",
	"l up ..\nf up/evil.txt",
	"l out ../outside\nf out/keep.txt",
	"l abs /tmp\nf abs/evil.txt",
	"d a\nl a/b ../..\nf a/b/evil.txt",
	"l x .\nl x/y ..\nf x/y/evil.txt",
	"f ok.txt\nh ../evil.txt ok.txt",
	"f ok.txt\nh link ../outside/keep.txt",
	"l twice inner\nd twice\nf twice/x",
	"f name\x00evil",
	"f \\\\server\\share\\evil",
	"l back ./../../x",
	"d dir\nl dir/self dir\nf dir/self/self/x",
}

func FuzzExtractStaysInside(f *testing.F) {
	for _, s := range slipScripts {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, script string) {
		if runtime.GOOS == "windows" && strings.Contains(script, "l ") {
			t.Skip("symlinks need extra rights on Windows")
		}
		parent, dest := sandbox(t)
		for _, clash := range []string{ClashBoth, ClashReplace, ClashSkip} {
			_, _ = Extract(fakeArchive{script}, dest, ExtractOptions{Clash: clash})
			checkContained(t, parent, dest)
		}
	})
}

func zipWith(t testing.TB, entries ...[2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e[0], Method: zip.Store}
		if strings.HasPrefix(e[1], "->") {
			h.SetMode(fs.ModeSymlink | 0o777)
			e[1] = strings.TrimPrefix(e[1], "->")
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, e[1])
	}
	zw.Close()
	return buf.Bytes()
}

func tarWith(t testing.TB, entries ...[2]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		h := &tar.Header{Name: e[0], Mode: 0o644, Size: int64(len(e[1])), Typeflag: tar.TypeReg}
		switch {
		case strings.HasPrefix(e[1], "->"):
			h.Typeflag, h.Linkname, h.Size = tar.TypeSymlink, strings.TrimPrefix(e[1], "->"), 0
		case strings.HasPrefix(e[1], "=>"):
			h.Typeflag, h.Linkname, h.Size = tar.TypeLink, strings.TrimPrefix(e[1], "=>"), 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			io.WriteString(tw, e[1])
		}
	}
	tw.Close()
	return buf.Bytes()
}

var slipEntries = [][][2]string{
	{{"../evil.txt", "x"}},
	{{"/abs/evil.txt", "x"}},
	{{"..\\evil.txt", "x"}},
	{{"a/../../evil.txt", "x"}},
	{{"up", "->.."}, {"up/evil.txt", "x"}},
	{{"out", "->../outside"}, {"out/keep.txt", "x"}},
	{{"ok", "x"}, {"hl", "=>../outside/keep.txt"}},
	{{"ok", "x"}, {"ok", "->../outside/keep.txt"}, {"ok", "overwrite through the link"}},
}

func fuzzReal(f *testing.F, ext string, seed func(testing.TB, ...[2]string) []byte) {
	for _, s := range slipEntries {
		f.Add(seed(f, s...))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need extra rights on Windows")
		}
		parent, dest := sandbox(t)
		file := filepath.Join(t.TempDir(), "a"+ext)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		a, err := Open(file, Options{})
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = Extract(a, dest, ExtractOptions{Clash: ClashReplace})
		checkContained(t, parent, dest)
	})
}

func FuzzZipStaysInside(f *testing.F) { fuzzReal(f, ".zip", zipWith) }

func FuzzTarStaysInside(f *testing.F) { fuzzReal(f, ".tar", tarWith) }

// The 7z and rar fuzzers start from the fixtures of other tools, as no Go
// writer for those formats exists to make crafted seeds with.
func fuzzFixtures(f *testing.F, ext string) {
	fixtures, _ := filepath.Glob(filepath.Join("testdata", "fixtures", "*"+ext))
	for _, p := range fixtures {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need extra rights on Windows")
		}
		parent, dest := sandbox(t)
		file := filepath.Join(t.TempDir(), "a"+ext)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		a, err := Open(file, Options{Password: "Bergkäse"})
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = Extract(a, dest, ExtractOptions{Clash: ClashReplace})
		checkContained(t, parent, dest)
	})
}

func Fuzz7zStaysInside(f *testing.F) { fuzzFixtures(f, ".7z") }

func FuzzRarStaysInside(f *testing.F) { fuzzFixtures(f, ".rar") }

func FuzzCleanName(f *testing.F) {
	for _, s := range []string{"a/b", "../x", "/x", "a\\..\\..\\x", "C:x", "", ".", "a/./b//c/"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		got, err := CleanName(name)
		if err != nil {
			return
		}
		if got == "" || strings.HasPrefix(got, "/") || strings.Contains(got, "\\") || strings.ContainsRune(got, 0) {
			t.Fatalf("CleanName(%q) = %q", name, got)
		}
		for _, seg := range strings.Split(got, "/") {
			if seg == ".." || seg == "." || seg == "" {
				t.Fatalf("CleanName(%q) = %q", name, got)
			}
		}
	})
}
