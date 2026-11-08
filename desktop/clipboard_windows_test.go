package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFilesCutHereComeBackCutFromTheClipboard(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "notes.txt"), filepath.Join(dir, "Grüße und Ölfässer.txt"), filepath.Join(dir, "sub")}
	for _, p := range paths[:2] {
		if err := os.WriteFile(p, []byte("x"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(paths[2], 0o777); err != nil {
		t.Fatal(err)
	}
	s := &shell{}
	t.Cleanup(func() { _ = s.ClearClipboard() })
	for _, cut := range []bool{true, false} {
		if err := s.CopyFiles(paths, cut); err != nil {
			t.Fatal(err)
		}
		got, gotCut, err := s.ClipboardFiles()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, paths) || gotCut != cut {
			t.Fatalf("got %v cut=%v, want %v cut=%v", got, gotCut, paths, cut)
		}
	}
}

func TestAClearedClipboardHoldsNoFiles(t *testing.T) {
	s := &shell{}
	if err := s.CopyFiles([]string{`C:\Windows`}, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearClipboard(); err != nil {
		t.Fatal(err)
	}
	if got, cut, err := s.ClipboardFiles(); err != nil || len(got) != 0 || cut {
		t.Fatalf("got %v cut=%v err=%v", got, cut, err)
	}
}
