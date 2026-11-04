package remotes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

// errNotRunning means rclone rcd does not answer, because it is missing,
// starting or restarting after a crash.
var errNotRunning = errors.New("rclone is not running")

// rcError is an error rclone reported for a call. Its text is rclone's
// own and usually says what is wrong with a remote.
type rcError struct {
	Method string
	Status int
	Msg    string
}

func (e *rcError) Error() string { return "rclone " + e.Method + ": " + e.Msg }

// rcClient calls the remote control API of rclone rcd.
type rcClient struct {
	http *http.Client
	base string
}

func socketClient(sock string) *rcClient {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	return &rcClient{http: &http.Client{Transport: tr}, base: "http://rclone/"}
}

// call posts in to method and decodes the answer into out. rclone wants a
// JSON object even for calls without parameters.
func (c *rcClient) call(ctx context.Context, method string, in, out any) error {
	if in == nil {
		in = struct{}{}
	}
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", errNotRunning, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) != nil || e.Error == "" {
			e.Error = resp.Status
		}
		return &rcError{Method: method, Status: resp.StatusCode, Msg: e.Error}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
