package elevate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

const pipePrefix = `\\.\pipe\cannoncommander-elevate-`

// overPipe is the line on Windows: a pipe of a fresh name that only this
// process serves, started as its first instance so nobody can have taken
// the name before, and taken only from the process start says it started.
// The helper checks in turn that the pipe belongs to this process, so the
// command line carries nothing secret.
func overPipe(ctx context.Context, start func(value string) (uint32, func() error, error)) (io.ReadWriteCloser, []byte, error) {
	raw := make([]byte, 16)
	rand.Read(raw)
	name := pipePrefix + hex.EncodeToString(raw)
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, nil, err
	}
	h, err := windows.CreateNamedPipe(p, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS, 1, 64<<10, 64<<10, 0, nil)
	if err != nil {
		return nil, nil, err
	}
	pid, wait, err := start(fmt.Sprintf("pipe:%s:%d", name, os.Getpid()))
	if err != nil {
		windows.CloseHandle(h)
		return nil, nil, err
	}
	connected := make(chan error, 1)
	go func() {
		err := windows.ConnectNamedPipe(h, nil)
		if errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			err = nil
		}
		connected <- err
	}()
	ended := make(chan error, 1)
	go func() { ended <- wait() }()
	fail := func(err error) (io.ReadWriteCloser, []byte, error) {
		// A caller of our own ends the wait for one that will not come.
		if c, cerr := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0); cerr == nil {
			windows.CloseHandle(c)
		}
		<-connected
		windows.CloseHandle(h)
		return nil, nil, err
	}
	select {
	case err := <-connected:
		if err != nil {
			windows.CloseHandle(h)
			return nil, nil, err
		}
	case err := <-ended:
		if errors.Is(err, ErrDeclined) {
			return fail(err)
		}
		return fail(ErrNoHelper)
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	var client uint32
	if err := windows.GetNamedPipeClientProcessId(h, &client); err != nil || client != pid {
		windows.CloseHandle(h)
		return nil, nil, ErrNoHelper
	}
	return os.NewFile(uintptr(h), name), nil, nil
}

// dialPipe is the helper's end. It takes the pipe only from the process that
// started it, and lets the server know no more of it than its identity.
func dialPipe(value string) (io.ReadWriteCloser, error) {
	cut := strings.LastIndex(value, ":")
	if cut < 0 || !strings.HasPrefix(value, pipePrefix) {
		return nil, ErrBadOp
	}
	name := value[:cut]
	parent, err := strconv.ParseUint(value[cut+1:], 10, 32)
	if err != nil {
		return nil, ErrBadOp
	}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
	if err != nil {
		return nil, err
	}
	var server uint32
	if err := windows.GetNamedPipeServerProcessId(h, &server); err != nil || server != uint32(parent) {
		windows.CloseHandle(h)
		return nil, errUntrusted
	}
	return os.NewFile(uintptr(h), name), nil
}
