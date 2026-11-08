package nativeshell

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// fakeBwrap stands in for bwrap: it notes its arguments next to itself,
// then runs the command after its options with the sandbox's /tmp turned
// back into the folder bound there.
const fakeBwrap = `#!/bin/sh
printf '%s\n' "$@" > "$(dirname "$0")/bwrap.log"
while [ "$1" != "--" ]; do
	case "$1" in
	--bind) tmp=$2; shift 3 ;;
	--ro-bind|--ro-bind-try|--symlink|--setenv) shift 3 ;;
	--proc|--dev|--chdir) shift 2 ;;
	*) shift ;;
	esac
done
shift
n=$#
for a do
	case "$a" in /tmp/input*|/tmp/thumbnail.png) a="$tmp/${a#/tmp/}" ;; esac
	set -- "$@" "$a"
done
shift $n
exec "$@"
`

func TestTheSandboxIsBuiltLikeGnomeDesktops(t *testing.T) {
	link := func(target string) *fstest.MapFile {
		return &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(target)}
	}
	file := &fstest.MapFile{Data: []byte("x")}
	head := []string{"/usr/bin/bwrap", "--ro-bind", "/usr", "/usr", "--ro-bind-try", "/etc/ld.so.cache", "/etc/ld.so.cache"}
	tail := []string{
		"--ro-bind-try", "/etc/alternatives", "/etc/alternatives", "--ro-bind-try", "/etc/fonts", "/etc/fonts",
		"--ro-bind-try", "/var/cache/fontconfig", "/var/cache/fontconfig",
		"--proc", "/proc", "--dev", "/dev", "--chdir", "/", "--unshare-all", "--die-with-parent", "--new-session", "--clearenv",
		"--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--setenv", "HOME", "/tmp", "--setenv", "GIO_USE_VFS", "local",
		"--bind", "/tmp/cc-thumbnail-1", "/tmp", "--ro-bind", "/home/u/a;rm -rf ~.png", "/tmp/input.png",
		"--", "gdk-pixbuf-thumbnailer", "-s", "256", "/tmp/input.png", "/tmp/thumbnail.png",
	}
	cases := []struct {
		name   string
		root   fstest.MapFS
		layout []string
	}{
		{"merged /usr", fstest.MapFS{
			"bin": link("usr/bin"), "lib": link("usr/lib"), "lib64": link("/usr/lib64"), "sbin": link("usr/sbin"),
			"usr/bin/true": file, "usr/lib/x": file, "usr/lib64/x": file, "usr/sbin/x": file,
		}, []string{"--symlink", "/usr/bin", "/bin", "--symlink", "/usr/lib64", "/lib64", "--symlink", "/usr/lib", "/lib", "--symlink", "/usr/sbin", "/sbin"}},
		{"split /usr", fstest.MapFS{
			"bin/sh": file, "lib/x": file, "lib32/x": file, "sbin/x": file, "lib64": link("lib"), "usr/bin/true": file,
		}, []string{"--ro-bind", "/bin", "/bin", "--ro-bind", "/lib64", "/lib64", "--ro-bind", "/lib", "/lib", "--ro-bind", "/lib32", "/lib32", "--ro-bind", "/sbin", "/sbin"}},
		{"only /usr", fstest.MapFS{"usr/bin/true": file, "lib32": file}, nil},
	}
	for _, c := range cases {
		got := bwrapArgs("/usr/bin/bwrap", c.root, "/tmp/cc-thumbnail-1", "/home/u/a;rm -rf ~.png", "/tmp/input.png",
			[]string{"gdk-pixbuf-thumbnailer", "-s", "256", "/tmp/input.png", "/tmp/thumbnail.png"})
		if want := slices.Concat(head, c.layout, tail); !slices.Equal(got, want) {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, want)
		}
	}
}

func TestWithoutASandboxNoThumbnailerRunsButTheCacheIsRead(t *testing.T) {
	f := newFixture(t)
	log := f.thumbnailer("exit 0")
	f.bwrap = ""
	files := f.files("new.png", "known.png")
	f.cached("normal", files[1], mtime(t, files[1]), 128)
	k := f.desk()
	if _, err := k.thumbnail(context.Background(), files[0], 100); !errors.Is(err, ErrNoPicture) {
		t.Errorf("got %v", err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("a thumbnailer ran")
	}
	if _, err := os.Stat(filepath.Join(f.home, ".cache", "thumbnails", "fail")); err == nil {
		t.Error("a failure was noted")
	}
	if b, err := k.thumbnail(context.Background(), files[1], 100); err != nil || size(t, b) != 128 {
		t.Errorf("the cache: %v", err)
	}
}

func TestBwrapIsTrustedOnlyAfterItRanOnce(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")
	script := func(name, status string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\necho \"$@\" >> "+shellQuote(runs)+"\nexit "+status+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	works, broken := script("works", "0"), script("broken", "1")
	look := func(p string) func(string) (string, error) {
		return func(name string) (string, error) {
			if name != "bwrap" || p == "" {
				return "", errors.New("not installed")
			}
			return p, nil
		}
	}
	root := fstest.MapFS{}
	if got := sandboxProgram(look(""), root); got != "" {
		t.Errorf("missing: got %s", got)
	}
	if got := sandboxProgram(look(broken), root); got != "" {
		t.Errorf("broken: got %s", got)
	}
	for range 2 {
		if got := sandboxProgram(look(works), root); got != works {
			t.Errorf("works: got %s", got)
		}
	}
	b, _ := os.ReadFile(runs)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[1], "--setenv GIO_USE_VFS local -- true") {
		t.Errorf("runs %q", lines)
	}
}
