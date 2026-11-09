package elevate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheProfileLetsOnlyTheAppsOwnProgramMakeUserNamespaces(t *testing.T) {
	text, err := usernsProfile("/opt/CannonCommander/CannonCommander")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"abi <abi/4.0>,\n",
		"include <tunables/global>\n",
		`profile cannoncommander-userns "/opt/CannonCommander/CannonCommander" flags=(unconfined) {`,
		"\n  userns,\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in\n%s", want, text)
		}
	}
	for _, exe := range []string{"", "CannonCommander", `/opt/a"b`, "/opt/a\nb", `/opt/a\b`, "/opt/{a}", "/opt/a\x00b"} {
		if _, err := usernsProfile(exe); !errors.Is(err, ErrBadOp) {
			t.Errorf("%q: got %v", exe, err)
		}
	}
}

// fakeParser stands in for apparmor_parser and notes its arguments.
func fakeParser(t *testing.T, status string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "parser.log")
	p := filepath.Join(dir, "apparmor_parser")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho \"$@\" >> '"+log+"'\nexit "+status+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p, log
}

func useProfileAt(t *testing.T, parser string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "cannoncommander-userns")
	oldFile, oldParsers := profileFile, parsers
	profileFile, parsers = file, []string{filepath.Join(t.TempDir(), "missing"), parser}
	t.Cleanup(func() { profileFile, parsers = oldFile, oldParsers })
	return file
}

func TestAllowingWritesTheProfileAndLoadsIt(t *testing.T) {
	parser, log := fakeParser(t, "0")
	file := useProfileAt(t, parser)
	if err := apply(Op{Kind: AllowUserNamespaces}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	if !strings.Contains(string(b), `"`+exe+`"`) {
		t.Errorf("the profile names another program:\n%s", b)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", info.Mode())
	}
	if err := apply(Op{Kind: DisallowUserNamespaces}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the profile stayed: %v", err)
	}
	calls, _ := os.ReadFile(log)
	if want := "-r " + file + "\n-R " + file + "\n"; string(calls) != want {
		t.Errorf("parser calls %q, want %q", calls, want)
	}
}

func TestAProfileTheParserRefusesIsNotLeftBehind(t *testing.T) {
	parser, _ := fakeParser(t, "1")
	file := useProfileAt(t, parser)
	if err := apply(Op{Kind: AllowUserNamespaces}); err == nil {
		t.Fatal("a refused profile counted as loaded")
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused profile stayed: %v", err)
	}
}

func TestDisallowingWithoutAProfileIsNoFailure(t *testing.T) {
	parser, log := fakeParser(t, "0")
	useProfileAt(t, parser)
	if err := apply(Op{Kind: DisallowUserNamespaces}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("the parser ran without a profile")
	}
}
