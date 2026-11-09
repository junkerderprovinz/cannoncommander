package remotes

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// TestResult is what a connection test found. Sizes are bytes and missing
// when the remote does not report them.
type TestResult struct {
	OK      bool   `json:"ok"`
	Entries int    `json:"entries"`
	Total   *int64 `json:"total,omitempty"`
	Used    *int64 `json:"used,omitempty"`
	Free    *int64 `json:"free,omitempty"`
	Error   string `json:"error,omitempty"`
	// Failure names the reason where the interface has a sentence for it.
	Failure string `json:"failure,omitempty"`
	// HostKey is the key the SFTP server showed, to compare with the one its
	// owner sees. Saving pins the remote to it.
	HostKey *HostKey `json:"hostKey,omitempty"`
	// Shares are what an SMB server offers, when the form names none yet.
	Shares []string `json:"shares,omitempty"`
	// Question is what rclone's setup wants chosen before it can go on.
	Question *Question `json:"question,omitempty"`
}

// Failures the interface explains in its own words.
const (
	failTimeout = "timeout"
	failHostKey = "hostKey"
	failMissing = "missing"
	failAddress = "address"
)

const testWait = 45 * time.Second

// addressKeys decide which server a connection reaches and as whom. A saved
// secret only joins a test whose form names the same ones, so a password
// never travels to a server the form names instead.
var addressKeys = []string{"host", "port", "user", "url", "endpoint", "auth", "auth_url", "token_url", "api_url"}

// Test tries a configuration without saving it and without writing to the
// remote. It lists the folder of the remote through a connection string,
// which leaves rclone.conf alone. Only a fresh OAuth token or a picked answer
// needs rclone's own setup, which runs on a section that lives for the test.
// With the name of a saved remote of the same type, the saved parameters
// fill in what the form left out, such as a password shown as unchanged.
func (s *Service) Test(ctx context.Context, in Input) (TestResult, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return TestResult{}, err
	}
	p, err := s.provider(ctx, in.Type)
	if err != nil {
		return TestResult{}, err
	}
	set, unset, token, picked, err := s.params(ctx, p, in.Params)
	if err != nil {
		return TestResult{}, err
	}
	freshToken := token != ""
	if r, ok, _ := s.db.get(ctx, strings.TrimSpace(in.Name)); ok && r.Type == in.Type {
		stored, err := s.section(ctx, r.Name)
		if err != nil {
			return TestResult{}, err
		}
		same := !slices.ContainsFunc(addressKeys, func(k string) bool {
			given, ok := set[k]
			return ok && given != stored[k]
		})
		for k, v := range stored {
			// Another server is no reason to trust the old one's key either.
			if _, given := set[k]; given || k == "type" || slices.Contains(unset, k) || ((p.secret(k) || k == hostKeysKey) && !same) {
				continue
			}
			set[k] = v
		}
		if token == "" && same {
			token = stored["token"]
		}
	}
	path := ""
	if in.Path != nil {
		path = strings.TrimSpace(*in.Path)
	}

	res := TestResult{}
	if in.Type == "sftp" && set["host"] != "" {
		if pin, ok := pinnedKey(set[hostKeysKey]); ok {
			res.HostKey = &pin
		} else if !checksHostKey(set) {
			key, err := s.readKey(ctx, set["host"], set["port"])
			if err != nil {
				return failed(res, err, "")
			}
			set[hostKeysKey] = key.Pin
			res.HostKey = &key
		}
	}

	base := connString(p, in.Type, set)
	if (freshToken && p.oauth()) || len(picked) > 0 {
		tmp := tryPrefix + randomID()
		defer s.forget(tmp)
		if err := s.createSection(ctx, tmp, in.Type, set, token, withSetupAnswers(in.Type, picked)); err != nil {
			if asks, ok := errors.AsType[errAsks](err); ok {
				res.Question = asks.q
				return res, nil
			}
			return failed(res, err, "")
		}
		base = tmp + ":"
	}

	listCtx, cancel := context.WithTimeout(ctx, testWait)
	defer cancel()
	var list struct {
		List []struct {
			Name  string `json:"Name"`
			IsDir bool   `json:"IsDir"`
		} `json:"list"`
	}
	err = s.rc.call(listCtx, "operations/list", map[string]any{
		"fs": base + path, "remote": "",
		"opt": map[string]any{"noModTime": true, "noMimeType": true},
	}, &list)
	switch {
	case err != nil && notFound(err) && path != "":
		return failed(res, scrub(err, base), failMissing)
	case err != nil && notFound(err) && (in.Type == "webdav" || in.Type == "http"):
		// A WebDAV root is the address itself, so nothing there means a
		// wrong address rather than an empty remote.
		return failed(res, scrub(err, base), failAddress)
	case err != nil && notFound(err):
	case err != nil:
		return failed(res, scrub(err, base), "")
	}
	res.OK, res.Entries = true, len(list.List)
	if in.Type == "smb" && path == "" {
		res.Shares = []string{}
		for _, e := range list.List {
			if e.IsDir {
				res.Shares = append(res.Shares, e.Name)
			}
		}
	}
	aboutCtx, cancelAbout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAbout()
	var about struct {
		Total *int64 `json:"total"`
		Used  *int64 `json:"used"`
		Free  *int64 `json:"free"`
	}
	if s.rc.call(aboutCtx, "operations/about", map[string]string{"fs": base}, &about) == nil {
		res.Total, res.Used, res.Free = about.Total, about.Used, about.Free
	}
	return res, nil
}

func notFound(err error) bool {
	rc, ok := errors.AsType[*rcError](err)
	return ok && strings.Contains(rc.Msg, "directory not found")
}

// failed turns what went wrong into a result. Only a missing rclone is an
// error of the request.
func failed(res TestResult, err error, why string) (TestResult, error) {
	var rc *rcError
	var p *v1.Problem
	var ne net.Error
	switch {
	case errors.Is(err, errNotRunning) || errors.Is(err, errUnavailable):
		return TestResult{}, err
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()):
		res.Error, res.Failure = "the remote did not answer in time", failTimeout
	case errors.As(err, &rc):
		res.Error = rc.Msg
	case errors.As(err, &p) && p.Status == http.StatusBadRequest:
		res.Error = p.Detail
	default:
		res.Error = err.Error()
	}
	if strings.Contains(res.Error, "host key mismatch") {
		res.Failure = failHostKey
	}
	if why != "" {
		res.Failure = why
	}
	return res, nil
}

// scrub takes the connection string out of what rclone says, since it
// carries the obscured password.
func scrub(err error, fs string) error {
	rc, ok := errors.AsType[*rcError](err)
	if !ok || !strings.HasPrefix(fs, ":") {
		return err
	}
	typ, _, _ := strings.Cut(strings.Trim(fs, ":"), ",")
	return &rcError{Method: rc.Method, Status: rc.Status, Msg: strings.ReplaceAll(rc.Msg, fs, ":"+typ+":")}
}

// connString is rclone's inline remote, :backend,k=v,k=v:, with the keys
// sorted. rclone reads it like a section of rclone.conf, so passwords go in
// obscured as params left them.
func connString(p *provider, typ string, params map[string]string) string {
	var b strings.Builder
	b.WriteString(":" + typ)
	for _, k := range slices.Sorted(maps.Keys(params)) {
		if _, ok := p.option(k); !ok || params[k] == "" {
			continue
		}
		b.WriteString("," + k + "=" + quoteValue(params[k]))
	}
	return b.String() + ":"
}

// quoteValue wraps a value in double quotes, doubling any inside, unless
// every character is safe bare. It allows rather than forbids, because the
// separators include the colon every URL has.
func quoteValue(v string) string {
	bare := v != "" && !strings.ContainsFunc(v, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && !strings.ContainsRune("-_.~", r)
	})
	if bare {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
}

func randomID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// OAuthInput carries the output of rclone authorize. With Name the token
// goes into that remote, otherwise it is checked and handed back for the
// remote the add dialog is about to create.
type OAuthInput struct {
	Type  string `json:"type"`
	Token string `json:"token"`
	Name  string `json:"name,omitempty"`
}

// OAuthResult is the checked token, or the remote that took it.
type OAuthResult struct {
	Token  string  `json:"token,omitempty"`
	Expiry string  `json:"expiry,omitempty"`
	Remote *Remote `json:"remote,omitempty"`
}

// OAuth takes a token from rclone authorize.
func (s *Service) OAuth(ctx context.Context, in OAuthInput) (OAuthResult, error) {
	token, expiry, err := normalizeToken(in.Token)
	if err != nil {
		return OAuthResult{}, err
	}
	if err := s.ready(ctx, readyWait); err != nil {
		return OAuthResult{}, err
	}
	typ := in.Type
	if in.Name != "" {
		r, err := s.row(ctx, in.Name)
		if err != nil {
			return OAuthResult{}, err
		}
		if typ == "" {
			typ = r.Type
		}
	}
	p, err := s.provider(ctx, typ)
	if err != nil {
		return OAuthResult{}, err
	}
	if !p.oauth() {
		return OAuthResult{}, v1.Errorf(http.StatusBadRequest, typ+" does not log in with a token")
	}
	if in.Name == "" {
		return OAuthResult{Token: token, Expiry: expiry}, nil
	}
	r, err := s.Update(ctx, in.Name, Input{Type: typ, Params: map[string]string{"token": token}})
	if err != nil {
		return OAuthResult{}, err
	}
	return OAuthResult{Expiry: expiry, Remote: &r}, nil
}

// normalizeToken finds the token in what rclone authorize printed, which
// wraps it in lines meant for a terminal.
func normalizeToken(raw string) (token, expiry string, err error) {
	bad := v1.Errorf(http.StatusBadRequest, "paste the token that rclone authorize printed, it starts with { and ends with }")
	i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if i < 0 || j < i {
		return "", "", bad
	}
	body := []byte(raw[i : j+1])
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	}
	if json.Unmarshal(body, &t) != nil || (t.AccessToken == "" && t.RefreshToken == "") {
		return "", "", bad
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		return "", "", bad
	}
	return buf.String(), t.Expiry, nil
}
