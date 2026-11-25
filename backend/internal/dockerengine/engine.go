// Package dockerengine speaks the Docker Engine API over its unix socket.
// Paths carry no API version, so the engine answers in its own.
package dockerengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Socket is where the engine listens when its socket is mounted into the
// container.
const Socket = "/var/run/docker.sock"

// Engine is one Docker engine.
type Engine struct {
	c    *http.Client
	base string
}

// Dial returns the engine behind the unix socket at path. Nothing is
// dialled before the first call.
func Dial(path string) *Engine {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", path)
	}}
	return New(&http.Client{Transport: tr, Timeout: 30 * time.Second}, "http://docker")
}

// New returns an engine reached at base through c, as a test server is.
func New(c *http.Client, base string) *Engine {
	return &Engine{c: c, base: base}
}

// ErrNotFound is the engine's 404, a container or network that went away.
var ErrNotFound = errors.New("not found in docker")

// Do sends body as JSON and decodes the answer into out, when either is set.
func (e *Engine) Do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.base+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := e.c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode >= 300 {
		var msg struct{ Message string }
		_ = json.NewDecoder(res.Body).Decode(&msg)
		return fmt.Errorf("docker %s %s: %d %s", method, path, res.StatusCode, msg.Message)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
