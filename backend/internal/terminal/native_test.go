//go:build desktop

package terminal

import (
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
)

// installed finds the programs it names, each in a folder of its own name.
func installed(names ...string) lookFunc {
	return func(name string) (string, error) {
		if slices.Contains(names, name) {
			return "/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestEachWordOpensItsProgramOnWindows(t *testing.T) {
	all := installed("cmd.exe", "powershell.exe", "pwsh.exe", "wt.exe", "bash.exe")
	for _, c := range []struct {
		kind, program string
		args          []string
		console       bool
	}{
		{"cmd", "/bin/cmd.exe", nil, true},
		{"powershell", "/bin/powershell.exe", nil, true},
		{"pwsh", "/bin/pwsh.exe", nil, true},
		{"wt", "/bin/wt.exe", []string{"-d", `C:\Work`}, false},
		{"terminal", "/bin/wt.exe", []string{"-d", `C:\Work`}, false},
		{"bash", "/bin/bash.exe", nil, true},
	} {
		t.Run(c.kind, func(t *testing.T) {
			l, err := nativeTerminal("windows", c.kind, `C:\Work`, "", all)
			if err != nil {
				t.Fatal(err)
			}
			if l.program != c.program || !slices.Equal(l.args, c.args) || l.console != c.console || l.dir != `C:\Work` {
				t.Fatalf("got %+v", l)
			}
		})
	}
}

func TestAFolderWithASemicolonStaysOneFolderForWindowsTerminal(t *testing.T) {
	l, err := nativeTerminal("windows", "wt", `C:\Temp\a;calc;b`, "", installed("wt.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"-d", `C:\Temp\a\;calc\;b`}; !slices.Equal(l.args, want) || l.dir != `C:\Temp\a;calc;b` {
		t.Fatalf("got %+v", l)
	}
}

func TestWithoutWindowsTerminalTheDefaultIsTheCommandPrompt(t *testing.T) {
	l, err := nativeTerminal("windows", "terminal", `C:\Work`, "", installed("cmd.exe", "powershell.exe"))
	if err != nil || l.program != "/bin/cmd.exe" || !l.console {
		t.Fatalf("got %+v, %v", l, err)
	}
}

func TestAProgramThatIsNotInstalledIsNamed(t *testing.T) {
	for kind, program := range map[string]string{"pwsh": "PowerShell 7", "wt": "Windows Terminal"} {
		_, err := nativeTerminal("windows", kind, `C:\Work`, "", installed("cmd.exe"))
		var p *v1.Problem
		if !errors.As(err, &p) || p.Code != "terminalMissing" || p.Params["program"] != program {
			t.Errorf("%s: got %v", kind, err)
		}
	}
}

func TestAMacAlwaysOpensTerminal(t *testing.T) {
	for _, kind := range Kinds {
		l, err := nativeTerminal("darwin", kind, "/Users/jdp/Work", "", installed())
		if err != nil || l.program != "open" || !slices.Equal(l.args, []string{"-a", "Terminal", "/Users/jdp/Work"}) {
			t.Fatalf("%s: got %+v, %v", kind, l, err)
		}
	}
}

func TestLinuxTakesTheUsersTerminalFirst(t *testing.T) {
	l, err := nativeTerminal("linux", "cmd", "/home/jdp", "alacritty", installed("alacritty", "x-terminal-emulator"))
	if err != nil || l.program != "/bin/alacritty" || l.dir != "/home/jdp" {
		t.Fatalf("got %+v, %v", l, err)
	}
	l, err = nativeTerminal("linux", "terminal", "/home/jdp", "", installed("konsole", "xterm"))
	if err != nil || l.program != "/bin/konsole" || !slices.Equal(l.args, []string{"--workdir", "/home/jdp"}) {
		t.Fatalf("got %+v, %v", l, err)
	}
	l, err = nativeTerminal("linux", "terminal", "/home/jdp", "", installed("gnome-terminal", "x-terminal-emulator"))
	if err != nil || l.program != "/bin/x-terminal-emulator" || l.args != nil {
		t.Fatalf("got %+v, %v", l, err)
	}
}

func TestLinuxWithoutATerminalSaysSo(t *testing.T) {
	_, err := nativeTerminal("linux", "terminal", "/home/jdp", "", installed())
	var p *v1.Problem
	if !errors.As(err, &p) || p.Code != "noTerminal" {
		t.Fatalf("got %v", err)
	}
}

func (e *env) native(body any) (int, string) {
	e.t.Helper()
	return e.do(e.request(http.MethodPost, "/terminal/native", body))
}

func TestTheDesktopOpensItsTerminalInTheFolder(t *testing.T) {
	e := newEnv(t)
	var started []launch
	e.h.pick = func(kind, dir string) (launch, error) { return launch{program: kind, dir: dir}, nil }
	e.h.start = func(l launch) error { started = append(started, l); return nil }

	if status, code := e.native(map[string]string{"path": "/mnt/user/work", "kind": "cmd"}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
	if len(started) != 1 || started[0].program != "cmd" || started[0].dir != filepath.Join(e.root, "work") {
		t.Fatalf("started %+v", started)
	}
	if got := e.recorded(); len(got) != 1 || got[0].EventType != activitydb.EventTerminalOpen || got[0].Path != "/mnt/user/work" {
		t.Fatalf("logged %+v", got)
	}
}

func TestTheDesktopTerminalNeedsNoSwitch(t *testing.T) {
	e := newEnv(t)
	e.switchOn(false)
	e.h.start = func(launch) error { return nil }
	e.h.pick = func(kind, dir string) (launch, error) { return launch{program: kind, dir: dir}, nil }
	if status, code := e.native(map[string]string{"path": "/mnt/user/work", "kind": "terminal"}); status != http.StatusNoContent {
		t.Fatalf("got %d %s", status, code)
	}
}

func TestTheNativeTerminalIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]string
		status int
		code   string
	}{
		{"for an unknown word", map[string]string{"path": "/mnt/user/work", "kind": "calc"}, http.StatusBadRequest, "terminalKind"},
		{"in a file", map[string]string{"path": "/mnt/user/work/notes.txt", "kind": "cmd"}, http.StatusBadRequest, "notAFolder"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.h.start = func(launch) error {
				t.Fatal("a terminal opened")
				return nil
			}
			if status, code := e.native(c.body); status != c.status || code != c.code {
				t.Fatalf("got %d %s", status, code)
			}
		})
	}
}

func TestAFlatpakSaysItCannotOpenTheComputersTerminal(t *testing.T) {
	t.Setenv("FLATPAK_ID", "io.github.junkerderprovinz.cannoncommander")
	if _, err := newNative().pick("terminal", "/home/jdp"); !errors.Is(err, errFlatpak) {
		t.Fatalf("got %v", err)
	}
}
