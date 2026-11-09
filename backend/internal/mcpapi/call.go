package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// A tool does its work through /api/v1 as the key's owner, so every check the
// web interface runs holds for an assistant as well: the scope, the access
// rules with hidden and read only folders, archives and the rights of the
// role. The request never leaves the process. Its context names the one
// operation it may reach, as the route its contract declares, which no
// request from the network can carry.
type principal struct {
	user *users.User
	op   string
}

type principalKey struct{}

var errOpNotAllowed = v1.Coded(http.StatusForbidden, "mcpNotAllowed", "this key cannot reach that operation")

// Authenticate puts the calls of the tools in front of next, the
// authenticator of /api/v1.
func Authenticate(next v1.Authenticator) v1.Authenticator {
	return func(w http.ResponseWriter, r *http.Request) (*users.User, error) {
		p, ok := r.Context().Value(principalKey{}).(*principal)
		if !ok {
			return next(w, r)
		}
		if r.Pattern != p.op {
			return nil, errOpNotAllowed
		}
		return p.user, nil
	}
}

// readOps are the operations a key without the write switch may reach. Every
// other one changes something.
var readOps = map[string]bool{
	"GET /fs/list":        true,
	"GET /fs/stat":        true,
	"GET /fs/text":        true,
	"GET /fs/tree":        true,
	"GET /search":         true,
	"GET /trash":          true,
	"GET /jobs/{id}":      true,
	"POST /ops/preflight": true,
}

// maxAnswer bounds what one call of /api/v1 may answer, which is far above a
// listing of a big folder and keeps a runaway one from filling the memory.
const maxAnswer = 64 << 20

var errAnswerTooBig = errors.New("the answer is too big")

// call runs one operation of /api/v1 as the caller's owner. pattern names the
// operation as its contract does, and target is the path that reaches it. A
// key without the write switch only gets to the operations in readOps; a
// content search, which starts a job, is the one exception, and search names
// it with the kind it starts.
func (s *Server) call(ctx context.Context, c *caller, method, pattern, target string, query url.Values, body, out any) error {
	op := method + " " + pattern
	if !c.key.CanWrite && !readOps[op] && !isContentSearch(op, body) {
		return errWriteOff
	}
	api := s.apiHandler()
	if api == nil {
		return errors.New("the MCP server is not mounted")
	}
	var rd *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	} else {
		rd = bytes.NewReader(nil)
	}
	u := &url.URL{Path: target, RawQuery: query.Encode()}
	ctx = context.WithValue(ctx, principalKey{}, &principal{user: c.user, op: op})
	ctx = v1.WithAgent(ctx, v1.Agent{KeyID: c.key.ID, Name: c.key.Label})
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return err
	}
	req.RemoteAddr = net.JoinHostPort(c.addr, "0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := &recorder{header: http.Header{}}
	api.ServeHTTP(rec, req)
	if rec.over {
		return errAnswerTooBig
	}
	if rec.status >= 300 {
		var p v1.Problem
		if err := json.Unmarshal(rec.buf.Bytes(), &p); err != nil || p.Status == 0 {
			p = *v1.Errorf(rec.status, "")
		}
		return &p
	}
	if out == nil || rec.buf.Len() == 0 {
		return nil
	}
	return json.Unmarshal(rec.buf.Bytes(), out)
}

func isContentSearch(op string, body any) bool {
	j, ok := body.(jobRequest)
	return ok && op == "POST /jobs" && j.Kind == kindContentSearch
}

// jobRequest is the body of POST /jobs.
type jobRequest struct {
	Kind   string `json:"kind"`
	Params any    `json:"params"`
}

const kindContentSearch = "contentSearch"

// recorder keeps the answer of a call of /api/v1 in memory, up to maxAnswer.
type recorder struct {
	header http.Header
	status int
	buf    bytes.Buffer
	over   bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if r.buf.Len()+len(b) > maxAnswer {
		r.over = true
		return 0, errAnswerTooBig
	}
	return r.buf.Write(b)
}
