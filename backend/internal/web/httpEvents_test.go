package web

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/events"
)

// pipeWriter streams what the handler writes so the test can read it live.
type pipeWriter struct {
	header http.Header
	w      *io.PipeWriter
}

func (p *pipeWriter) Header() http.Header         { return p.header }
func (p *pipeWriter) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p *pipeWriter) WriteHeader(int)             {}
func (p *pipeWriter) Flush()                      {}

func openStream(t *testing.T, ctx context.Context, username string) *bufio.Scanner {
	t.Helper()
	pr, pw := io.Pipe()
	w := &pipeWriter{header: http.Header{}, w: pw}
	r := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	d := &Context{
		User: &users.User{FrontendUser: users.FrontendUser{Username: username, Permissions: users.Permissions{Realtime: true}}},
		Ctx:  ctx,
	}
	go func() {
		_, _ = SSEHandler(w, r, d)
		pw.Close()
	}()
	t.Cleanup(func() { pr.Close() })
	s := bufio.NewScanner(pr)
	if !nextData(t, s, "acknowledge") {
		t.Fatal("stream did not start")
	}
	return s
}

func nextData(t *testing.T, s *bufio.Scanner, want string) bool {
	t.Helper()
	found := make(chan bool, 1)
	go func() {
		for s.Scan() {
			if strings.Contains(s.Text(), want) {
				found <- true
				return
			}
		}
		found <- false
	}()
	select {
	case ok := <-found:
		return ok
	case <-time.After(2 * time.Second):
		return false
	}
}

func TestBroadcastReachesEverySSEClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := openStream(t, ctx, "sse-first")
	second := openStream(t, ctx, "sse-second")

	events.Broadcast("onlyOfficeLog", `"hello both"`)

	if !nextData(t, first, "hello both") {
		t.Error("first client missed the broadcast")
	}
	if !nextData(t, second, "hello both") {
		t.Error("second client missed the broadcast")
	}
}
