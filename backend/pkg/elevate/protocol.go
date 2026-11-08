package elevate

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrDeclined is a prompt for administrator rights that was turned down.
	ErrDeclined = errors.New("administrator rights were not given")
	// ErrNoHelper is a helper that never called back.
	ErrNoHelper = errors.New("the helper with administrator rights did not answer")
	// ErrUnsupported says that this system has no way to ask for the rights.
	ErrUnsupported = errors.New("administrator rights cannot be asked for here")

	errUntrusted = errors.New("the other side does not know the secret")
)

// OpError is the failure of one operation as the helper reported it.
type OpError struct {
	Code string
	Msg  string
}

func (e *OpError) Error() string { return e.Msg }

// maxOps keeps a job to what one selection in the app can hold.
const maxOps = 100000

// connectWait is how long the parent waits for the helper once the prompt is
// answered, and how long the helper takes at most to get its job.
var connectWait = 30 * time.Second

// launch starts the helper with administrator rights and returns a function
// that waits for it to end. It blocks while the prompt is open.
var launch = launchHelper

// The messages, one JSON value each. The helper proves first that it knows
// the secret, the parent then signs the job with it, and the helper answers
// every operation in order and ends with done.
type hello struct {
	Proof string `json:"proof"`
}

type job struct {
	Ops []Op   `json:"ops"`
	MAC string `json:"mac"`
}

type result struct {
	I     int    `json:"i"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
	Done  bool   `json:"done,omitempty"`
}

func sign(secret []byte, what string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(what))
	return m.Sum(nil)
}

func proven(secret []byte, what, proof string) bool {
	got, err := hex.DecodeString(proof)
	return err == nil && hmac.Equal(got, sign(secret, what))
}

func jobText(ops []Op) (string, error) {
	raw, err := json.Marshal(ops)
	return "job\n" + string(raw), err
}

// Run has the helper do ops with administrator rights and calls report once
// per operation, in order, with nil or an *OpError. The error it returns
// means that the helper did not run or broke off; operations it did not
// report did not happen.
func Run(ctx context.Context, ops []Op, report func(i int, err error)) error {
	if len(ops) == 0 || len(ops) > maxOps {
		return ErrBadOp
	}
	for _, o := range ops {
		if err := o.check(); err != nil {
			return err
		}
	}
	text, err := jobText(ops)
	if err != nil {
		return err
	}
	secret := make([]byte, 32)
	rand.Read(secret)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	found := make(chan net.Conn, 1)
	go welcome(ln, secret, found)
	wait, err := launch(ctx, ln.Addr().(*net.TCPAddr).Port, secret)
	if err != nil {
		return err
	}
	ended := make(chan error, 1)
	go func() { ended <- wait() }()

	var conn net.Conn
	select {
	case conn = <-found:
	case err := <-ended:
		if errors.Is(err, ErrDeclined) {
			return err
		}
		return ErrNoHelper
	case <-time.After(connectWait):
		return ErrNoHelper
	case <-ctx.Done():
		return ctx.Err()
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := json.NewEncoder(conn).Encode(job{Ops: ops, MAC: hex.EncodeToString(sign(secret, text))}); err != nil {
		return err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	next := 0
	for next < len(ops) {
		var r result
		if err := dec.Decode(&r); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return ErrNoHelper
		}
		if r.Done || r.I != next {
			break
		}
		if r.Error != "" {
			report(next, &OpError{Code: r.Code, Msg: r.Error})
		} else {
			report(next, nil)
		}
		next++
	}
	if next < len(ops) {
		return ErrNoHelper
	}
	return nil
}

// welcome hands on the first connection that proves the secret. Any other
// program on the computer can connect to the port too, so others are closed
// and the wait goes on.
func welcome(ln net.Listener, secret []byte, found chan<- net.Conn) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		var h hello
		if err := json.NewDecoder(io.LimitReader(c, 4096)).Decode(&h); err != nil || !proven(secret, "helper", h.Proof) {
			c.Close()
			continue
		}
		c.SetReadDeadline(time.Time{})
		found <- c
		return
	}
}

// Serve is the helper. arg is the parent's port, followed on Windows by a
// colon and the secret in hex; elsewhere the secret comes as the first line
// of stdin, since the command line of a process is public there.
func Serve(arg string, stdin io.Reader) error {
	port, key, _ := strings.Cut(arg, ":")
	if key == "" && stdin != nil {
		line, err := bufio.NewReader(io.LimitReader(stdin, 256)).ReadString('\n')
		if err != nil && line == "" {
			return err
		}
		key = line
	}
	secret, err := hex.DecodeString(strings.TrimSpace(key))
	if err != nil || len(secret) < 16 {
		return errUntrusted
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return ErrBadOp
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(connectWait))
	enc := json.NewEncoder(conn)
	if err := enc.Encode(hello{Proof: hex.EncodeToString(sign(secret, "helper"))}); err != nil {
		return err
	}
	var j job
	if err := json.NewDecoder(io.LimitReader(conn, 64<<20)).Decode(&j); err != nil {
		return err
	}
	text, err := jobText(j.Ops)
	if err != nil || !proven(secret, text, j.MAC) {
		return errUntrusted
	}
	if len(j.Ops) == 0 || len(j.Ops) > maxOps {
		return ErrBadOp
	}
	conn.SetDeadline(time.Time{})
	for i, op := range j.Ops {
		r := result{I: i}
		if err := apply(op); err != nil {
			r.Error, r.Code = err.Error(), codeOf(err)
		}
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return enc.Encode(result{Done: true})
}

// InProcess makes Run hand its jobs to a helper in this process, with the
// rights the process has, for the tests of the packages that use Run. It
// returns the function that undoes it.
func InProcess() (undo func()) {
	was := launch
	launch = func(_ context.Context, port int, secret []byte) (func() error, error) {
		done := make(chan error, 1)
		go func() { done <- Serve(fmt.Sprintf("%d:%x", port, secret), nil) }()
		return func() error { return <-done }, nil
	}
	return func() { launch = was }
}
