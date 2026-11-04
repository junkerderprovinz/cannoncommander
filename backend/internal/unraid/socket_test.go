//go:build unix

package unraid

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// fakeEmhttpd answers on a unix socket like emhttpd: empty on success, an
// error text when the csrf token is wrong.
func fakeEmhttpd(t *testing.T, path string) func() []string {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var got []string
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if r.Method != http.MethodPost || r.URL.Path != "/update" || err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if form.Get("csrf_token") != "T0KEN" {
			_, _ = io.WriteString(w, "invalid csrf_token")
			return
		}
		mu.Lock()
		got = append(got, string(body))
		mu.Unlock()
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// shortSocket returns a socket path short enough for the 108 byte limit,
// which a nested test directory can exceed.
func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func TestSpindownTalksToEmhttpd(t *testing.T) {
	e := newEnv(t)
	e.cfg.Write = true
	e.cfg.Socket = shortSocket(t)
	e.svc = New(e.cfg)
	got := fakeEmhttpd(t, e.cfg.Socket)

	done, err := e.svc.Spindown(context.Background(), []string{"disk2", "disk3", "disk2"})
	if err != nil || !slices.Equal(done, []string{"disk2", "disk3"}) {
		t.Fatalf("spindown = %v, %v", done, err)
	}
	if _, err := e.svc.Spindown(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"cmdSpindown=disk2&csrf_token=T0KEN", "cmdSpindown=disk3&csrf_token=T0KEN", "cmdSpindownAll=Apply&csrf_token=T0KEN"}
	if !slices.Equal(got(), want) {
		t.Errorf("emhttpd got %q", got())
	}
	if _, err := e.svc.Spindown(context.Background(), []string{"disk9"}); !errors.Is(err, ErrUnknownDisk) {
		t.Errorf("unknown disk: %v", err)
	}

	e.writeVar("csrf_token=\"stale\"\n")
	if _, err := e.svc.Spindown(context.Background(), []string{"disk2"}); err == nil || err.Error() != "emhttpd: invalid csrf_token" {
		t.Errorf("rejected command: %v", err)
	}
}
