package nativeshell

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// fixture is a Linux desktop laid out in a temporary folder: the user's
// home with its data and config folders, and a data folder of the system.
type fixture struct {
	t                  *testing.T
	home, user, system string
	config, work       string
	installed          []string
	langs              []string
	// root stands in for / where /proc and /sys are read, owners are the
	// names owned on the session bus, and output answers for programs.
	root   fstest.MapFS
	owners []string
	output func(program string, args ...string) ([]byte, error)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{
		t: t, home: filepath.Join(base, "home"), system: filepath.Join(base, "usr", "share"),
		installed: []string{"present"}, langs: []string{"de_AT", "de"},
	}
	f.user = filepath.Join(f.home, ".local", "share")
	f.config = filepath.Join(f.home, ".config")
	f.work = filepath.Join(f.home, "work")
	f.write(filepath.Join(f.system, "mime", "globs2"), "50:text/plain:*.txt\n50:image/png:*.png\n50:image/jpeg:*.jpg\n50:audio/mpeg:*.mp3\n50:video/mp4:*.mp4\n"+
		"50:image/svg+xml:*.svg\n50:application/x-shellscript:*.sh\n50:application/pdf:*.pdf\n")
	f.write(filepath.Join(f.system, "mime", "subclasses"), "image/svg+xml application/xml\napplication/xml text/plain\napplication/x-shellscript text/plain\n")
	f.write(filepath.Join(f.system, "mime", "aliases"), "image/jpg image/jpeg\n")
	return f
}

func (f *fixture) write(file, content string) string {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return file
}

func (f *fixture) script(file, content string) string {
	f.t.Helper()
	f.write(file, content)
	if err := os.Chmod(file, 0o755); err != nil {
		f.t.Fatal(err)
	}
	return file
}

// files makes files in the work folder; a name ending in / is a folder.
func (f *fixture) files(names ...string) []string {
	f.t.Helper()
	var out []string
	for _, n := range names {
		p := filepath.Join(f.work, n)
		if strings.HasSuffix(n, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				f.t.Fatal(err)
			}
		} else {
			f.write(p, "content of "+n)
		}
		out = append(out, p)
	}
	return out
}

func (f *fixture) desktop() xdg.Desktop {
	return xdg.Desktop{
		Data: []string{f.user, f.system}, Config: []string{f.config}, Cache: filepath.Join(f.home, ".cache"), Home: f.home, Langs: f.langs,
		Look: func(name string) (string, error) {
			if slices.Contains(f.installed, name) {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not installed")
		},
	}
}

// desk reads the current desktop.
func (f *fixture) desk() *desk {
	d := f.desktop()
	d.Output = f.output
	k := newDesk(d, nil)
	k.root = f.root
	k.hasOwner = func(name string) bool { return slices.Contains(f.owners, name) }
	return k
}

// install makes the desktop the one the exported functions see.
func (f *fixture) install() *desk {
	k := f.desk()
	deskMu.Lock()
	shared = k
	deskMu.Unlock()
	f.t.Cleanup(func() {
		deskMu.Lock()
		shared = nil
		deskMu.Unlock()
	})
	return k
}

func (f *fixture) selection(k *desk, names ...string) selection {
	f.t.Helper()
	sel, err := k.selection(f.files(names...))
	if err != nil {
		f.t.Fatal(err)
	}
	return sel
}

func keys(actions []action) string {
	var out []string
	for _, a := range actions {
		out = append(out, a.Key)
	}
	return strings.Join(out, " ")
}

// waitFor waits for a file a started program writes.
func waitFor(t *testing.T, file string) string {
	t.Helper()
	for range 100 {
		if b, err := os.ReadFile(file); err == nil && strings.HasSuffix(string(b), "end\n") {
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s was not written", file)
	return ""
}

func TestASelectionKnowsTypesAndFolders(t *testing.T) {
	f := newFixture(t)
	sel := f.selection(f.desk(), "a.png", "photos/", "notes.zzz")
	if sel.dir != f.work {
		t.Errorf("dir %s", sel.dir)
	}
	var got []string
	for _, it := range sel.items {
		got = append(got, it.mime)
	}
	if !slices.Equal(got, []string{"image/png", "inode/directory", "text/plain"}) {
		t.Errorf("types %q", got)
	}
	if _, err := f.desk().selection([]string{filepath.Join(f.work, "gone")}); err == nil {
		t.Error("a missing file was selected")
	}
}

func TestATypeMatchesItselfItsParentsAndWildcards(t *testing.T) {
	f := newFixture(t)
	k := f.desk()
	svg := f.selection(k, "a.svg").items[0]
	cases := map[string]bool{"image/svg+xml": true, "image/*": true, "text/plain": true, "application/xml": true, "*": true, "image/png": false, "text/*": true, "video/*": false}
	for pattern, want := range cases {
		if got := k.is(svg, pattern); got != want {
			t.Errorf("%s: got %v", pattern, got)
		}
	}
	jpg := f.selection(k, "b.jpg").items[0]
	if !k.is(jpg, "image/jpg") {
		t.Error("an alias does not match")
	}
}
