package elevate

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain lets the test binary be the helper, as the desktop app is.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--elevated" {
		if err := Serve(os.Args[2], os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func inProcess(t *testing.T) { t.Cleanup(InProcess()) }

type outcome struct {
	mu   sync.Mutex
	errs map[int]error
}

func (o *outcome) report(i int, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.errs == nil {
		o.errs = map[int]error{}
	}
	o.errs[i] = err
}

func TestAJobRunsThroughTheHelper(t *testing.T) {
	inProcess(t)
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a.txt"), "a")
	ops := []Op{
		{Kind: Copy, From: filepath.Join(dir, "a.txt"), To: filepath.Join(dir, "b.txt")},
		{Kind: Delete, From: filepath.Join(dir, "fehlt")},
		{Kind: Mkdir, To: filepath.Join(dir, "neu")},
	}
	var o outcome
	if err := Run(context.Background(), ops, o.report); err != nil {
		t.Fatal(err)
	}
	if len(o.errs) != 3 || o.errs[0] != nil || o.errs[2] != nil {
		t.Fatalf("results %v", o.errs)
	}
	var oe *OpError
	if !errors.As(o.errs[1], &oe) || oe.Code != "notFound" {
		t.Fatalf("the missing file: %v", o.errs[1])
	}
	if read(t, filepath.Join(dir, "b.txt")) != "a" {
		t.Fatal("not copied")
	}
	if _, err := os.Stat(filepath.Join(dir, "neu")); err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesABadOpBeforeAskingForRights(t *testing.T) {
	asked := false
	was := launch
	launch = func(context.Context, int, []byte) (func() error, error) { asked = true; return nil, errors.New("no") }
	t.Cleanup(func() { launch = was })
	err := Run(context.Background(), []Op{{Kind: Delete, From: "relative"}}, func(int, error) {})
	if !errors.Is(err, ErrBadOp) || asked {
		t.Fatalf("got %v, asked %v", err, asked)
	}
}

func TestADeclinedPromptEndsTheRun(t *testing.T) {
	was := launch
	launch = func(context.Context, int, []byte) (func() error, error) { return nil, ErrDeclined }
	t.Cleanup(func() { launch = was })
	err := Run(context.Background(), []Op{{Kind: Mkdir, To: filepath.Join(t.TempDir(), "x")}}, func(int, error) {})
	if !errors.Is(err, ErrDeclined) {
		t.Fatalf("got %v", err)
	}
}

func TestAHelperThatNeverCallsEndsTheRun(t *testing.T) {
	was, wait := launch, connectWait
	launch = func(context.Context, int, []byte) (func() error, error) { return func() error { return nil }, nil }
	connectWait = time.Second
	t.Cleanup(func() { launch, connectWait = was, wait })
	err := Run(context.Background(), []Op{{Kind: Mkdir, To: filepath.Join(t.TempDir(), "x")}}, func(int, error) {})
	if !errors.Is(err, ErrNoHelper) {
		t.Fatalf("got %v", err)
	}
}

func TestTheParentTurnsAwayAStrangerAndWaitsForTheHelper(t *testing.T) {
	dir := t.TempDir()
	was := launch
	launch = func(_ context.Context, port int, secret []byte) (func() error, error) {
		stranger, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
		json.NewEncoder(stranger).Encode(hello{Proof: hex.EncodeToString(sign([]byte("geraten"), "helper"))})
		var j job
		if err := json.NewDecoder(stranger).Decode(&j); err == nil {
			t.Error("a stranger got the job")
		}
		stranger.Close()
		done := make(chan error, 1)
		go func() { done <- Serve(fmt.Sprintf("%d:%x", port, secret), nil) }()
		return func() error { return <-done }, nil
	}
	t.Cleanup(func() { launch = was })
	var o outcome
	if err := Run(context.Background(), []Op{{Kind: Mkdir, To: filepath.Join(dir, "x")}}, o.report); err != nil {
		t.Fatal(err)
	}
	if o.errs[0] != nil {
		t.Fatal(o.errs[0])
	}
}

func TestTheHelperDoesNothingForAJobWithoutTheSecret(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	secret := []byte("0123456789abcdef0123456789abcdef")
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var h hello
		json.NewDecoder(c).Decode(&h)
		ops := []Op{{Kind: Mkdir, To: filepath.Join(dir, "boese")}}
		raw, _ := json.Marshal(ops)
		json.NewEncoder(c).Encode(job{Ops: ops, MAC: hex.EncodeToString(sign([]byte("falsch"), "job\n"+string(raw)))})
		c.Read(make([]byte, 64))
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := Serve(fmt.Sprintf("%d:%x", port, secret), nil); !errors.Is(err, errUntrusted) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "boese")); err == nil {
		t.Fatal("the helper did the job")
	}
}

func TestTheHelperRunsAsAProcessOfItsOwn(t *testing.T) {
	was := launch
	launch = func(ctx context.Context, port int, secret []byte) (func() error, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "--elevated", strconv.Itoa(port))
		cmd.Stdin = strings.NewReader(hex.EncodeToString(secret) + "\n")
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd.Wait, nil
	}
	t.Cleanup(func() { launch = was })
	dir := t.TempDir()
	file(t, filepath.Join(dir, "a", "x.txt"), "x")
	var o outcome
	if err := Run(context.Background(), []Op{{Kind: Move, From: filepath.Join(dir, "a"), To: filepath.Join(dir, "b")}}, o.report); err != nil {
		t.Fatal(err)
	}
	if o.errs[0] != nil || read(t, filepath.Join(dir, "b", "x.txt")) != "x" {
		t.Fatalf("results %v", o.errs)
	}
}

// realPrompt runs ops through the real launcher. It needs someone to answer
// the prompt: a person, an account that is elevated without one, or an agent
// such as pkttyagent, which CC_TEST_ELEVATE_WAIT gives time to register for
// this process before the prompt comes.
func realPrompt(t *testing.T, want string, ops []Op) error {
	t.Helper()
	if os.Getenv("CC_TEST_ELEVATE") != want {
		t.Skip("asks for administrator rights; set CC_TEST_ELEVATE=" + want)
	}
	if wait, err := time.ParseDuration(os.Getenv("CC_TEST_ELEVATE_WAIT")); err == nil {
		fmt.Println("waiting for an agent, pid", os.Getpid())
		time.Sleep(wait)
	}
	var o outcome
	err := Run(context.Background(), ops, o.report)
	if err == nil && o.errs[0] != nil {
		t.Fatal(o.errs[0])
	}
	return err
}

func TestElevatedForReal(t *testing.T) {
	dir := t.TempDir()
	if err := realPrompt(t, "1", []Op{{Kind: Mkdir, To: filepath.Join(dir, "als-admin")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "als-admin")); err != nil {
		t.Fatal(err)
	}
}

func TestElevatedForRealCanBeTurnedDown(t *testing.T) {
	dir := t.TempDir()
	if err := realPrompt(t, "decline", []Op{{Kind: Mkdir, To: filepath.Join(dir, "als-admin")}}); !errors.Is(err, ErrDeclined) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "als-admin")); err == nil {
		t.Fatal("the helper did the job")
	}
}
