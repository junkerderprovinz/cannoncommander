//go:build desktop

package terminal

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
)

// The desktop app runs on the user's own computer, so instead of a shell in
// the browser it opens that computer's terminal in a window of its own. Each
// word the path field knows names a program; terminal is the one a user of
// the platform expects.

// Kinds are the words the path field opens a terminal for.
var Kinds = []string{"cmd", "sh", "bash", "terminal", "powershell", "pwsh", "wt"}

var (
	errKind       = v1.Coded(http.StatusBadRequest, "terminalKind", "kind is one of cmd, sh, bash, terminal, powershell, pwsh or wt")
	errMissing    = v1.Coded(http.StatusNotFound, "terminalMissing", "that program is not installed here")
	errNoTerminal = v1.Coded(http.StatusNotFound, "noTerminal", "no terminal program was found on this computer")
)

// launch is a terminal program to start in a folder.
type launch struct {
	program string
	args    []string
	dir     string
	// console asks Windows for a console window of its own, which cmd and
	// PowerShell need to show up at all.
	console bool
}

// native opens the computer's own terminal: pick picks the program for a
// word and a folder, and start starts it.
type native struct {
	pick  func(kind, dir string) (launch, error)
	start func(launch) error
}

func newNative() native {
	return native{
		pick: func(kind, dir string) (launch, error) {
			return nativeTerminal(runtime.GOOS, kind, dir, os.Getenv("TERMINAL"), exec.LookPath)
		},
		start: startNative,
	}
}

// lookFunc finds a program by name the way exec.LookPath does.
type lookFunc func(name string) (string, error)

// nativeTerminal returns how to open the terminal kind names in dir on goos.
// term is the TERMINAL variable, which some Linux desktops set to the
// user's choice.
func nativeTerminal(goos, kind, dir, term string, look lookFunc) (launch, error) {
	switch goos {
	case "windows":
		return windowsTerminal(kind, dir, look)
	case "darwin":
		// Terminal.app comes with every Mac, and open hands it the folder.
		return launch{program: "open", args: []string{"-a", "Terminal", dir}, dir: dir}, nil
	}
	return linuxTerminal(dir, term, look)
}

func windowsTerminal(kind, dir string, look lookFunc) (launch, error) {
	console := func(name string) (launch, error) {
		p, err := look(name)
		if err != nil {
			return launch{}, err
		}
		return launch{program: p, dir: dir, console: true}, nil
	}
	// Windows Terminal takes the folder as an argument and opens the
	// profile its user made the default.
	wt := func() (launch, error) {
		p, err := look("wt.exe")
		if err != nil {
			return launch{}, err
		}
		return launch{program: p, args: []string{"-d", dir}, dir: dir}, nil
	}
	missing := func(program string) error { return errMissing.With("program", program) }
	switch kind {
	case "cmd":
		return console("cmd.exe")
	case "powershell":
		return console("powershell.exe")
	case "pwsh":
		if l, err := console("pwsh.exe"); err == nil {
			return l, nil
		}
		return launch{}, missing("PowerShell 7")
	case "wt":
		if l, err := wt(); err == nil {
			return l, nil
		}
		return launch{}, missing("Windows Terminal")
	case "bash", "sh":
		// Git for Windows or WSL, whichever put a bash on the PATH.
		if l, err := console("bash.exe"); err == nil {
			return l, nil
		}
	}
	if l, err := wt(); err == nil {
		return l, nil
	}
	return console("cmd.exe")
}

// linuxTerminals are tried in this order after TERMINAL. Debian and Ubuntu
// point x-terminal-emulator at the one the user chose. Each entry is the
// program and the arguments that open it in a folder; those without any
// start where they are started.
var linuxTerminals = []struct {
	name string
	args func(dir string) []string
}{
	{"x-terminal-emulator", nil},
	{"gnome-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"kgx", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"konsole", func(d string) []string { return []string{"--workdir", d} }},
	{"xfce4-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"mate-terminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"lxterminal", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"tilix", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"terminator", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"kitty", func(d string) []string { return []string{"--directory", d} }},
	{"alacritty", func(d string) []string { return []string{"--working-directory", d} }},
	{"wezterm", func(d string) []string { return []string{"start", "--cwd", d} }},
	{"foot", func(d string) []string { return []string{"--working-directory=" + d} }},
	{"xterm", nil},
}

func linuxTerminal(dir, term string, look lookFunc) (launch, error) {
	if term != "" {
		if p, err := look(term); err == nil {
			return launch{program: p, dir: dir}, nil
		}
	}
	for _, t := range linuxTerminals {
		p, err := look(t.name)
		if err != nil {
			continue
		}
		l := launch{program: p, dir: dir}
		if t.args != nil {
			l.args = t.args(dir)
		}
		return l, nil
	}
	return launch{}, errNoTerminal
}

// openNative is POST /terminal/native.
func (h *handlers) openNative(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	}
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if !slices.Contains(Kinds, in.Kind) {
		return errKind
	}
	u := v1.UserFrom(r)
	loc, err := h.folder(u, in.Path)
	if err != nil {
		return err
	}
	l, err := h.pick(in.Kind, loc.Real)
	if err != nil {
		return err
	}
	if err := h.start(l); err != nil {
		return err
	}
	h.record(r, u, activitydb.Entry{EventType: activitydb.EventTerminalOpen, Path: loc.Path})
	logger.Infof("opened %s in %s", filepath.Base(l.program), loc.Real)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
