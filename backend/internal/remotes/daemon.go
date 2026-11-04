package remotes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// daemon keeps rclone rcd running as a child process and starts it again
// when it dies. onUp runs every time it answers, first on the first start.
type daemon struct {
	cfg  Config
	rc   *rcClient
	onUp func(ctx context.Context, first bool)

	mu      sync.Mutex
	up      bool
	version string
	failure string
	changed chan struct{}
}

func newDaemon(cfg Config, rc *rcClient) *daemon {
	return &daemon{cfg: cfg, rc: rc, changed: make(chan struct{})}
}

// status returns whether rcd answers, its version and why it last failed.
func (d *daemon) status() (up bool, version, failure string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.up, d.version, d.failure
}

func (d *daemon) set(up bool, version, failure string) {
	d.mu.Lock()
	d.up, d.version, d.failure = up, version, failure
	close(d.changed)
	d.changed = make(chan struct{})
	d.mu.Unlock()
}

// wait blocks until rcd answers or ctx ends.
func (d *daemon) wait(ctx context.Context) error {
	for {
		d.mu.Lock()
		up, failure, changed := d.up, d.failure, d.changed
		d.mu.Unlock()
		if up {
			return nil
		}
		select {
		case <-ctx.Done():
			if failure != "" {
				return fmt.Errorf("%w: %s", errNotRunning, failure)
			}
			return errNotRunning
		case <-changed:
		}
	}
}

// run keeps rcd alive until ctx ends. A crash right after the start backs
// off up to a minute, so a broken setup does not spin.
func (d *daemon) run(ctx context.Context) {
	delay := time.Second
	for first := true; ctx.Err() == nil; first = false {
		began := time.Now()
		err := d.once(ctx, first)
		if ctx.Err() != nil {
			d.set(false, "", "")
			return
		}
		d.set(false, "", err.Error())
		logger.Errorf("rclone rcd stopped, starting it again: %v", err)
		if time.Since(began) > time.Minute {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, time.Minute)
	}
}

func (d *daemon) once(ctx context.Context, first bool) error {
	for _, dir := range []string{d.cfg.Dir, d.cfg.cacheDir(), filepath.Dir(d.cfg.Socket)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	_ = os.Remove(d.cfg.Socket)
	cmd := exec.CommandContext(ctx, d.cfg.Binary, "rcd",
		"--rc-addr", "unix://"+d.cfg.Socket,
		// The socket sits in a folder only this process can open.
		"--rc-no-auth",
		"--config", d.cfg.confFile(),
		"--cache-dir", d.cfg.cacheDir(),
		"--log-level", "NOTICE",
	)
	cmd.SysProcAttr = childAttr()
	cmd.Cancel = func() error { return stopChild(cmd.Process) }
	// Unmounting on SIGTERM takes rcd a moment.
	cmd.WaitDelay = 15 * time.Second
	out := &logLines{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	version, err := d.answers(ctx, exited)
	if err != nil {
		_ = stopChild(cmd.Process)
		<-exited
		return err
	}
	d.set(true, version, "")
	logger.Infof("rclone %s is running", version)
	go d.onUp(ctx, first)
	if err := <-exited; err != nil {
		return err
	}
	return fmt.Errorf("rclone rcd exited")
}

// answers waits until rcd takes calls on its socket.
func (d *daemon) answers(ctx context.Context, exited <-chan error) (string, error) {
	deadline := time.After(30 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			return "", fmt.Errorf("rclone rcd exited before it answered: %v", err)
		case <-deadline:
			return "", fmt.Errorf("rclone rcd did not answer within 30 seconds")
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
			var v struct {
				Version string `json:"version"`
			}
			callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := d.rc.call(callCtx, "core/version", nil, &v)
			cancel()
			if err == nil {
				return strings.TrimPrefix(v.Version, "v"), nil
			}
		}
	}
}

var logStamp = regexp.MustCompile(`^\d{4}/\d\d/\d\d \d\d:\d\d:\d\d `)

// logLines passes rclone's output on to the server log line by line.
type logLines struct {
	mu  sync.Mutex
	buf []byte
}

func (l *logLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(logStamp.ReplaceAllString(string(l.buf[:i]), ""))
		l.buf = l.buf[i+1:]
		switch {
		case line == "":
		case strings.HasPrefix(line, "ERROR") || strings.HasPrefix(line, "CRITICAL"):
			logger.Errorf("rclone: %s", line)
		default:
			logger.Infof("rclone: %s", line)
		}
	}
	return len(p), nil
}
