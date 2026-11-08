package elevate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"golang.org/x/sys/windows"
)

// childOverPipe starts this test binary as the helper, without other rights,
// and tells the parent the process id claim makes of the real one.
func childOverPipe(t *testing.T, claim func(uint32) uint32, seen *string) {
	t.Helper()
	was := launch
	launch = func(ctx context.Context) (io.ReadWriteCloser, []byte, error) {
		return overPipe(ctx, func(value string) (uint32, func() error, error) {
			if seen != nil {
				*seen = value
			}
			cmd := exec.Command(os.Args[0], "--elevated", value)
			if err := cmd.Start(); err != nil {
				return 0, nil, err
			}
			return claim(uint32(cmd.Process.Pid)), cmd.Wait, nil
		})
	}
	t.Cleanup(func() { launch = was })
}

func TestTheHelperCallsBackThroughAPipe(t *testing.T) {
	var value string
	childOverPipe(t, func(pid uint32) uint32 { return pid }, &value)
	dir := t.TempDir()
	var o outcome
	if err := Run(context.Background(), []Op{{Kind: Mkdir, To: filepath.Join(dir, "x")}}, o.report); err != nil || o.errs[0] != nil {
		t.Fatalf("got %v, %v", err, o.errs)
	}
	if !regexp.MustCompile(`^pipe:` + regexp.QuoteMeta(pipePrefix) + `[0-9a-f]{32}:[0-9]+$`).MatchString(value) {
		t.Fatalf("the command line holds %q", value)
	}
}

func TestAStrangerOnThePipeIsTurnedAway(t *testing.T) {
	childOverPipe(t, func(pid uint32) uint32 { return pid + 4 }, nil)
	dir := t.TempDir()
	err := Run(context.Background(), []Op{{Kind: Mkdir, To: filepath.Join(dir, "x")}}, func(int, error) {})
	if !errors.Is(err, ErrNoHelper) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err == nil {
		t.Fatal("the stranger did the job")
	}
}

func TestTheHelperOnlyTalksToItsParent(t *testing.T) {
	name := fmt.Sprintf("%stest%d", pipePrefix, os.Getpid())
	p, _ := windows.UTF16PtrFromString(name)
	h, err := windows.CreateNamedPipe(p, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT, 1, 4096, 4096, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	go windows.ConnectNamedPipe(h, nil)
	err = Serve(fmt.Sprintf("pipe:%s:%d", name, os.Getpid()+4), nil)
	if !errors.Is(err, errUntrusted) {
		t.Fatalf("got %v", err)
	}
}
