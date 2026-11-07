//go:build !windows

package terminal

import (
	"bytes"
	"cmp"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type shell struct {
	cmd  *exec.Cmd
	pty  *os.File
	done chan struct{}
}

// start runs a shell in dir on a new pseudo terminal. It leads a session of
// its own, so stop finds every job it started.
func start(dir string, sz size) (tty, error) {
	cmd, err := command(dir)
	if err != nil {
		return nil, err
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: sz.cols, Rows: sz.rows})
	if err != nil {
		return nil, err
	}
	sh := &shell{cmd: cmd, pty: f, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(sh.done)
	}()
	return sh, nil
}

func (sh *shell) Read(p []byte) (int, error)  { return sh.pty.Read(p) }
func (sh *shell) Write(p []byte) (int, error) { return sh.pty.Write(p) }
func (sh *shell) exited() <-chan struct{}     { return sh.done }

func (sh *shell) resize(sz size) error {
	return pty.Setsize(sh.pty, &pty.Winsize{Cols: sz.cols, Rows: sz.rows})
}

// stop hangs up on the shell and its jobs as closing a terminal window does,
// and kills whatever is left a second later.
func (sh *shell) stop() {
	sid := sh.cmd.Process.Pid
	signalSession(sid, syscall.SIGHUP)
	_ = sh.pty.Close()
	select {
	case <-sh.done:
	case <-time.After(time.Second):
	}
	signalSession(sid, syscall.SIGKILL)
	<-sh.done
}

// signalSession sends sig to every process group in the session sid. A shell
// with job control puts each job into a group of its own.
func signalSession(sid int, sig syscall.Signal) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		_ = syscall.Kill(-sid, sig)
		return
	}
	groups := map[int]bool{}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// The name in parentheses may hold spaces, so the fields count from
		// its end: state, parent, group, session.
		f := strings.Fields(string(raw[bytes.LastIndexByte(raw, ')')+1:]))
		if len(f) < 4 || f[3] != strconv.Itoa(sid) {
			continue
		}
		if g, err := strconv.Atoi(f[2]); err == nil {
			groups[g] = true
		}
	}
	for g := range groups {
		_ = syscall.Kill(-g, sig)
	}
}

func command(dir string) (*exec.Cmd, error) {
	sh := "/bin/sh"
	if _, err := os.Stat("/bin/bash"); err == nil {
		sh = "/bin/bash"
	}
	args := []string{sh}
	// The container hands the server capabilities such as reading every
	// file, which a shell would inherit. It gets the plain user instead.
	if ambientCaps() {
		setpriv, err := exec.LookPath("setpriv")
		if err != nil {
			return nil, errors.New("setpriv is missing to drop the server's capabilities for the shell")
		}
		args = append([]string{setpriv, "--inh-caps=-all", "--ambient-caps=-all", "--"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = environ(sh)
	return cmd, nil
}

func ambientCaps() bool {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "CapAmb:"); ok {
			return strings.Trim(strings.TrimSpace(v), "0") != ""
		}
	}
	return false
}

// environ is what the shell starts with. The server's own variables stay
// out, as they may carry secrets. History stays in memory, so what is typed
// is not written to the data folder, which is the home of the container's user.
func environ(sh string) []string {
	home := "/"
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		home = u.HomeDir
	}
	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=" + cmp.Or(os.Getenv("LANG"), "C.UTF-8"),
		"HOME=" + home,
		"SHELL=" + sh,
		"HISTFILE=",
		`PS1=\w \$ `,
	}
	if tz := os.Getenv("TZ"); tz != "" {
		env = append(env, "TZ="+tz)
	}
	return env
}
