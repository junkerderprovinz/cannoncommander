//go:build !windows

package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
)

type term struct {
	t    *testing.T
	conn *websocket.Conn
	out  chan string
	err  chan error
}

func (e *env) dial(path string) (*term, error) {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	target := strings.Replace(e.srv.URL, "http", "ws", 1) + "/terminal?cols=100&rows=30&path=" + url.QueryEscape(path)
	h := http.Header{}
	h.Set("X-User", "jdp")
	h.Set("Cookie", "s=session-1")
	conn, resp, err := websocket.Dial(ctx, target, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		if resp != nil {
			return nil, errors.New(resp.Status)
		}
		return nil, err
	}
	tm := &term{t: e.t, conn: conn, out: make(chan string, 256), err: make(chan error, 1)}
	go func() {
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				tm.err <- err
				close(tm.out)
				return
			}
			tm.out <- string(data)
		}
	}()
	e.t.Cleanup(func() { conn.CloseNow() })
	return tm, nil
}

func (tm *term) send(s string) {
	tm.t.Helper()
	if err := tm.conn.Write(context.Background(), websocket.MessageBinary, []byte(s)); err != nil {
		tm.t.Fatal(err)
	}
}

func (tm *term) resize(cols, rows int) {
	tm.t.Helper()
	raw, _ := json.Marshal(map[string]int{"cols": cols, "rows": rows})
	if err := tm.conn.Write(context.Background(), websocket.MessageText, raw); err != nil {
		tm.t.Fatal(err)
	}
}

// waitFor reads output until it matches the pattern and returns everything read.
func (tm *term) waitFor(pattern string) string {
	tm.t.Helper()
	re := regexp.MustCompile(pattern)
	var seen strings.Builder
	deadline := time.After(10 * time.Second)
	for {
		select {
		case s, ok := <-tm.out:
			if !ok {
				tm.t.Fatalf("closed before %s came, saw %q", pattern, seen.String())
			}
			seen.WriteString(s)
			if re.MatchString(seen.String()) {
				return seen.String()
			}
		case <-deadline:
			tm.t.Fatalf("no %s within 10s, saw %q", pattern, seen.String())
		}
	}
}

// closed waits for the server to end the terminal and returns its close status and reason.
func (tm *term) closed() (websocket.StatusCode, string) {
	tm.t.Helper()
	for {
		select {
		case <-tm.out:
		case err := <-tm.err:
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				return ce.Code, ce.Reason
			}
			return websocket.CloseStatus(err), ""
		case <-time.After(10 * time.Second):
			tm.t.Fatal("the terminal stayed open")
		}
	}
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func TestTypingReachesTheShellAndComesBack(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	tm.send("echo round''trip; pwd\n")
	tm.waitFor("roundtrip")
	tm.waitFor(`(?m)^` + regexp.QuoteMeta(filepath.Join(e.root, "work")) + `\r?$`)
}

func TestTheShellFollowsTheWindowSize(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	tm.send("stty size\n")
	tm.waitFor("30 100")
	tm.resize(132, 43)
	tm.send("stty size\n")
	tm.waitFor("43 132")
}

func TestExitClosesTheTerminalAndLogsBothEnds(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	tm.send("echo secret-typing\nexit\n")
	if code, reason := tm.closed(); code != websocket.StatusNormalClosure || reason != "exited" {
		t.Fatalf("got %d %q", code, reason)
	}
	var kinds []activitydb.EventType
	deadline := time.Now().Add(5 * time.Second)
	for len(kinds) < 2 && time.Now().Before(deadline) {
		kinds = kinds[:0]
		for _, en := range e.recorded() {
			kinds = append(kinds, en.EventType)
			if en.Path != "/mnt/user/work" {
				t.Errorf("%s names %q", en.EventType, en.Path)
			}
			raw, _ := json.Marshal(en)
			if strings.Contains(string(raw), "secret-typing") {
				t.Errorf("the log holds what was typed: %s", raw)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(kinds) != 2 || kinds[0] != activitydb.EventTerminalOpen || kinds[1] != activitydb.EventTerminalClose {
		t.Fatalf("logged %v", kinds)
	}
}

func TestEachUserHasAFewTerminalsAtMost(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	for i := range maxPerUser {
		tm, err := e.dial("/mnt/user/work")
		if err != nil {
			t.Fatalf("terminal %d: %v", i+1, err)
		}
		tm.send("echo ready\n")
		tm.waitFor("ready")
	}
	if status, code := e.check("/mnt/user/work"); status != http.StatusTooManyRequests || code != "terminalLimit" {
		t.Fatalf("the check let one more through: %d %s", status, code)
	}
	if _, err := e.dial("/mnt/user/work"); err == nil {
		t.Fatal("one more terminal opened")
	}
}

func TestAnIdleTerminalEnds(t *testing.T) {
	e := newEnv(t)
	e.h.idle = 300 * time.Millisecond
	e.h.every = 50 * time.Millisecond
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	if code, reason := tm.closed(); code != closeIdle || reason != "idle" {
		t.Fatalf("got %d %q", code, reason)
	}
}

func TestTheTerminalEndsWhenAccessDoes(t *testing.T) {
	for _, c := range []struct {
		name   string
		take   func(e *env)
		reason string
	}{
		{"revoked session", func(e *env) { e.gate.revoke("session-1") }, "signedOut"},
		{"no admin any more", func(e *env) { e.demote("jdp") }, "denied"},
		{"switched off", func(e *env) { e.switchOn(false) }, "off"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.h.every = 50 * time.Millisecond
			e.switchOn(true)
			tm, err := e.dial("/mnt/user/work")
			if err != nil {
				t.Fatal(err)
			}
			tm.send("echo ready\n")
			tm.waitFor("ready")
			c.take(e)
			if code, reason := tm.closed(); code != closeAccess || reason != c.reason {
				t.Fatalf("got %d %q", code, reason)
			}
		})
	}
}

func TestShutdownEndsTheShellWithItsJobs(t *testing.T) {
	e := newEnv(t)
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	tm.send("sleep 300 & echo job=$!; echo shell=$$\n")
	found := regexp.MustCompile(`job=(\d+)[\s\S]*shell=(\d+)`).FindStringSubmatch(tm.waitFor(`job=\d+[\s\S]*shell=\d+\s`))
	var pids []int
	for _, v := range found[1:] {
		n, _ := strconv.Atoi(v)
		pids = append(pids, n)
	}
	e.h.pool.Close()
	if code, _ := tm.closed(); code != websocket.StatusGoingAway {
		t.Fatalf("closed with %d", code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, pid := range pids {
		for alive(pid) && !zombie(pid) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if alive(pid) && !zombie(pid) {
			t.Errorf("process %d outlived the server", pid)
		}
	}
}

// zombie reports a process that ended and waits for its parent to collect it,
// which a test binary does not do for its grandchildren.
func zombie(pid int) bool {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	i := strings.LastIndexByte(string(raw), ')')
	return i > 0 && strings.HasPrefix(strings.TrimSpace(string(raw[i+1:])), "Z")
}

func TestTheShellGetsACleanEnvironment(t *testing.T) {
	t.Setenv("CC_SECRET_FOR_TEST", "leaked")
	e := newEnv(t)
	e.switchOn(true)
	tm, err := e.dial("/mnt/user/work")
	if err != nil {
		t.Fatal(err)
	}
	tm.send("echo \"term=$TERM secret=${CC_SECRET_FOR_TEST:-none}\"\n")
	tm.waitFor("term=xterm-256color secret=none")
}
