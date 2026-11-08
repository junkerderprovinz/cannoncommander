package elevate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func file(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOnlyPlainAbsolutePathsPass(t *testing.T) {
	dir := t.TempDir()
	root := filepath.VolumeName(dir) + string(filepath.Separator)
	bad := []Op{
		{Kind: "format", From: filepath.Join(dir, "a")},
		{Kind: Delete},
		{Kind: Delete, From: "relative/a"},
		{Kind: Delete, From: dir + string(filepath.Separator) + ".." + string(filepath.Separator) + "a"},
		{Kind: Delete, From: root},
		{Kind: Delete, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b")},
		{Kind: Copy, From: filepath.Join(dir, "a")},
		{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "a", "b")},
		{Kind: Mkdir, From: filepath.Join(dir, "a")},
		{Kind: Delete, From: filepath.Join(dir, "a\x00b")},
	}
	if runtime.GOOS == "windows" {
		bad = append(bad, Op{Kind: Delete, From: `\\server\share\a`}, Op{Kind: Delete, From: `\\?\C:\a`}, Op{Kind: Delete, From: `\\.\PhysicalDrive0`})
	}
	for _, op := range bad {
		if err := op.check(); !errors.Is(err, ErrBadOp) {
			t.Errorf("%+v passed: %v", op, err)
		}
	}
	good := []Op{
		{Kind: Delete, From: filepath.Join(dir, "a")},
		{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b")},
		{Kind: Mkdir, To: filepath.Join(dir, "neu")},
	}
	for _, op := range good {
		if err := op.check(); err != nil {
			t.Errorf("%+v: %v", op, err)
		}
	}
}

func TestCopyTakesAFolderWithEverythingInIt(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "x.txt"), "x")
	file(t, filepath.Join(dir, "a", "tief", "y.txt"), "y")
	if err := apply(Op{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b")}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "b", "tief", "y.txt")) != "y" || read(t, filepath.Join(dir, "a", "x.txt")) != "x" {
		t.Fatal("the copy is not whole")
	}
}

func TestCopyLeavesWhatIsThereUnlessToldToReplace(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a.txt"), "neu")
	file(t, filepath.Join(dir, "ziel", "a.txt"), "alt")
	op := Op{Kind: Copy, From: filepath.Join(dir, "a.txt"), To: filepath.Join(dir, "ziel", "a.txt")}
	if err := apply(op); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("got %v", err)
	}
	op.Replace = true
	if err := apply(op); err != nil {
		t.Fatal(err)
	}
	if read(t, op.To) != "neu" {
		t.Fatal("not replaced")
	}
}

func TestReplacingAFolderMergesIt(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "neu.txt"), "neu")
	file(t, filepath.Join(dir, "a", "beide.txt"), "von a")
	file(t, filepath.Join(dir, "b", "alt.txt"), "alt")
	file(t, filepath.Join(dir, "b", "beide.txt"), "von b")
	if err := apply(Op{Kind: Move, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Replace: true}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "b", "alt.txt")) != "alt" || read(t, filepath.Join(dir, "b", "neu.txt")) != "neu" || read(t, filepath.Join(dir, "b", "beide.txt")) != "von a" {
		t.Fatal("the folders were not merged")
	}
	if _, err := os.Lstat(filepath.Join(dir, "a")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the moved folder is still there")
	}
}

func TestMoveDeleteAndMkdir(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "x.txt"), "x")
	if err := apply(Op{Kind: Move, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b")}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "b", "x.txt")) != "x" {
		t.Fatal("not moved")
	}
	if err := apply(Op{Kind: Delete, From: filepath.Join(dir, "b")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "b")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("not deleted")
	}
	if err := apply(Op{Kind: Mkdir, To: filepath.Join(dir, "neu")}); err != nil {
		t.Fatal(err)
	}
	if err := apply(Op{Kind: Mkdir, To: filepath.Join(dir, "neu")}); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("a second mkdir: %v", err)
	}
	if err := apply(Op{Kind: Delete, From: filepath.Join(dir, "fehlt")}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleting what is not there: %v", err)
	}
}

func TestCopyNeedsAnExistingFolderToLandIn(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a.txt"), "a")
	if err := apply(Op{Kind: Copy, From: filepath.Join(dir, "a.txt"), To: filepath.Join(dir, "fehlt", "a.txt")}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteLeavesTheTargetOfALinkAlone(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "ziel", "wichtig.txt"), "x")
	if err := os.Symlink(filepath.Join(dir, "ziel"), filepath.Join(dir, "link")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if err := apply(Op{Kind: Delete, From: filepath.Join(dir, "link")}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "ziel", "wichtig.txt")) != "x" {
		t.Fatal("the link was followed")
	}
}

func TestCodesNameTheCommonFailures(t *testing.T) {
	for err, want := range map[error]string{
		&fs.PathError{Err: fs.ErrPermission}: "denied",
		&fs.PathError{Err: fs.ErrExist}:      "nameTaken",
		&fs.PathError{Err: fs.ErrNotExist}:   "notFound",
		ErrBadOp:                             "badPath",
		errors.New("anything"):               "",
	} {
		if got := codeOf(err); got != want {
			t.Errorf("%v: %q", err, got)
		}
	}
}
