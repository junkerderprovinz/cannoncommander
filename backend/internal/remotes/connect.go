package remotes

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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
}

// Test tries a configuration without saving it. It lists the folder of the
// remote through a section that lives for the test only. With the name of
// a saved remote of the same type, the saved parameters fill in what the
// form left out, such as a password shown as unchanged.
func (s *Service) Test(ctx context.Context, in Input) (TestResult, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return TestResult{}, err
	}
	p, err := s.provider(ctx, in.Type)
	if err != nil {
		return TestResult{}, err
	}
	set, unset, token, err := s.params(ctx, p, in.Params)
	if err != nil {
		return TestResult{}, err
	}
	if r, ok, _ := s.db.get(ctx, strings.TrimSpace(in.Name)); ok && r.Type == in.Type {
		stored, err := s.section(ctx, r.Name)
		if err != nil {
			return TestResult{}, err
		}
		for k, v := range stored {
			if _, given := set[k]; !given && k != "type" && !slices.Contains(unset, k) {
				set[k] = v
			}
		}
		if token == "" {
			token = stored["token"]
		}
	}
	path := ""
	if in.Path != nil {
		path = strings.TrimSpace(*in.Path)
	}
	tmp := tryPrefix + randomID()
	defer s.forget(tmp)
	if err := s.createSection(ctx, tmp, in.Type, set, token); err != nil {
		return failedTest(err)
	}
	listCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var list struct {
		List []json.RawMessage `json:"list"`
	}
	err = s.rc.call(listCtx, "operations/list", map[string]any{
		"fs": tmp + ":" + path, "remote": "",
		"opt": map[string]any{"noModTime": true, "noMimeType": true},
	}, &list)
	if err != nil {
		return failedTest(err)
	}
	res := TestResult{OK: true, Entries: len(list.List)}
	aboutCtx, cancelAbout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAbout()
	var about struct {
		Total *int64 `json:"total"`
		Used  *int64 `json:"used"`
		Free  *int64 `json:"free"`
	}
	if s.rc.call(aboutCtx, "operations/about", map[string]string{"fs": tmp + ":"}, &about) == nil {
		res.Total, res.Used, res.Free = about.Total, about.Used, about.Free
	}
	return res, nil
}

// failedTest turns what rclone said about a configuration into a result.
// Only a missing rclone is an error of the request.
func failedTest(err error) (TestResult, error) {
	var rc *rcError
	var p *v1.Problem
	switch {
	case errors.As(err, &rc):
		return TestResult{Error: rc.Msg}, nil
	case errors.As(err, &p) && p.Status == http.StatusBadRequest:
		return TestResult{Error: p.Detail}, nil
	case errors.Is(err, context.DeadlineExceeded):
		return TestResult{Error: "the remote did not answer in time"}, nil
	}
	return TestResult{}, err
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
