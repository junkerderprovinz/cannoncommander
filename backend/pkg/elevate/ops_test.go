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

func TestALinkIsNotDeletedWithRights(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "ziel", "wichtig.txt"), "x")
	linkDir(t, filepath.Join(dir, "ziel"), filepath.Join(dir, "link"))
	if err := apply(Op{Kind: Delete, From: filepath.Join(dir, "link")}); err == nil {
		t.Fatal("the link was deleted")
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

func TestCopyLeavesTheSkippedEntriesOut(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "offen.txt"), "o")
	file(t, filepath.Join(dir, "a", "geheim.txt"), "g")
	file(t, filepath.Join(dir, "a", "privat", "x.txt"), "x")
	op := Op{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Skip: []string{filepath.Join(dir, "a", "geheim.txt"), filepath.Join(dir, "a", "privat")}}
	if err := apply(op); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "b", "offen.txt")) != "o" {
		t.Fatal("the open file is missing")
	}
	for _, p := range []string{"geheim.txt", "privat"} {
		if _, err := os.Lstat(filepath.Join(dir, "b", p)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s was copied", p)
		}
	}
}

func TestMoveLeavesTheSkippedEntriesWhereTheyAre(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "offen.txt"), "o")
	file(t, filepath.Join(dir, "a", "tief", "geheim.txt"), "g")
	file(t, filepath.Join(dir, "a", "tief", "auch.txt"), "a")
	op := Op{Kind: Move, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Skip: []string{filepath.Join(dir, "a", "tief", "geheim.txt")}}
	if err := apply(op); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "b", "offen.txt")) != "o" || read(t, filepath.Join(dir, "b", "tief", "auch.txt")) != "a" {
		t.Fatal("not moved")
	}
	if read(t, filepath.Join(dir, "a", "tief", "geheim.txt")) != "g" {
		t.Fatal("the skipped file did not stay")
	}
	if _, err := os.Lstat(filepath.Join(dir, "b", "tief", "geheim.txt")); err == nil {
		t.Fatal("the skipped file went along")
	}
	if _, err := os.Lstat(filepath.Join(dir, "a", "offen.txt")); err == nil {
		t.Fatal("a moved file stayed")
	}
}

func TestSkipStaysInsideTheSource(t *testing.T) {
	dir := t.TempDir()
	for _, skip := range [][]string{{filepath.Join(dir, "anderswo")}, {filepath.Join(dir, "a")}, {"relativ"}} {
		op := Op{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Skip: skip}
		if err := op.check(); !errors.Is(err, ErrBadOp) {
			t.Errorf("%v passed: %v", skip, err)
		}
	}
	if err := (Op{Kind: Delete, From: filepath.Join(dir, "a"), Skip: []string{filepath.Join(dir, "a", "x")}}).check(); !errors.Is(err, ErrBadOp) {
		t.Error("a delete took a skip list")
	}
}

func TestParentsMakesTheFolderToLandIn(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a.txt"), "a")
	if err := apply(Op{Kind: Move, From: filepath.Join(dir, "a.txt"), To: filepath.Join(dir, "neu", "tief", "a.txt"), Parents: true}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dir, "neu", "tief", "a.txt")) != "a" {
		t.Fatal("not moved into the new folder")
	}
}

func TestNothingIsDoneThroughALinkOnTheWay(t *testing.T) {
	dir, victim := t.TempDir(), t.TempDir()
	file(t, filepath.Join(victim, "wichtig.txt"), "w")
	linkDir(t, victim, filepath.Join(dir, "link"))
	file(t, filepath.Join(dir, "a.txt"), "a")
	for _, op := range []Op{
		{Kind: Delete, From: filepath.Join(dir, "link", "wichtig.txt")},
		{Kind: Delete, From: filepath.Join(dir, "link")},
		{Kind: Copy, From: filepath.Join(dir, "a.txt"), To: filepath.Join(dir, "link", "a.txt")},
		{Kind: Move, From: filepath.Join(dir, "link", "wichtig.txt"), To: filepath.Join(dir, "b.txt")},
		{Kind: Mkdir, To: filepath.Join(dir, "link", "neu")},
	} {
		if err := apply(op); err == nil {
			t.Errorf("%+v went through", op)
		}
	}
	if read(t, filepath.Join(victim, "wichtig.txt")) != "w" {
		t.Fatal("the file behind the link changed")
	}
	for _, p := range []string{"a.txt", "neu"} {
		if _, err := os.Lstat(filepath.Join(victim, p)); err == nil {
			t.Errorf("%s landed behind the link", p)
		}
	}
}

func TestAMergeNeverWritesThroughALinkAtTheTarget(t *testing.T) {
	dir, victim := t.TempDir(), t.TempDir()
	file(t, filepath.Join(dir, "a", "sub", "x.txt"), "neu")
	file(t, filepath.Join(victim, "x.txt"), "alt")
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir(t, victim, filepath.Join(dir, "b", "sub"))
	err := apply(Op{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Replace: true})
	if err == nil {
		t.Error("the merge went into the link")
	}
	if read(t, filepath.Join(victim, "x.txt")) != "alt" {
		t.Fatal("a file behind the link was replaced")
	}
}

func TestReplacingAFileLinkReplacesTheLinkItself(t *testing.T) {
	dir, victim := t.TempDir(), t.TempDir()
	file(t, filepath.Join(dir, "a", "x.txt"), "neu")
	file(t, filepath.Join(victim, "x.txt"), "alt")
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(victim, "x.txt"), filepath.Join(dir, "b", "x.txt")); err != nil {
		t.Skip("no file links here:", err)
	}
	if err := apply(Op{Kind: Copy, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b"), Replace: true}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(victim, "x.txt")) != "alt" {
		t.Fatal("the file behind the link was written")
	}
	if read(t, filepath.Join(dir, "b", "x.txt")) != "neu" {
		t.Fatal("the copy is missing")
	}
}

func TestAMoveWithinOneVolumeIsARename(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "tief", "x.txt"), "x")
	file(t, filepath.Join(dir, "b.txt"), "b")
	file(t, filepath.Join(dir, "ziel", "b.txt"), "alt")
	before := map[string]os.FileInfo{}
	for _, p := range []string{"a", "b.txt"} {
		fi, err := os.Lstat(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		// Windows reads a file's identity by its path on first use, and the
		// path is gone after the move.
		os.SameFile(fi, fi)
		before[p] = fi
	}
	ops := []Op{
		{Kind: Move, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "ziel", "a")},
		{Kind: Move, From: filepath.Join(dir, "b.txt"), To: filepath.Join(dir, "ziel", "b.txt"), Replace: true},
	}
	for _, op := range ops {
		if err := apply(op); err != nil {
			t.Fatal(err)
		}
	}
	for p, fi := range before {
		after, err := os.Lstat(filepath.Join(dir, "ziel", p))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(fi, after) {
			t.Errorf("%s was copied, not renamed", p)
		}
	}
	if read(t, filepath.Join(dir, "ziel", "b.txt")) != "b" || read(t, filepath.Join(dir, "ziel", "a", "tief", "x.txt")) != "x" {
		t.Fatal("not moved")
	}
}

func TestARenameDoesNotReplaceUnlessTold(t *testing.T) {
	dir := t.TempDir()
	file(t, filepath.Join(dir, "b.txt"), "neu")
	file(t, filepath.Join(dir, "ziel", "b.txt"), "alt")
	if err := apply(Op{Kind: Move, From: filepath.Join(dir, "b.txt"), To: filepath.Join(dir, "ziel", "b.txt")}); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("got %v", err)
	}
	if read(t, filepath.Join(dir, "ziel", "b.txt")) != "alt" {
		t.Fatal("replaced")
	}
}
