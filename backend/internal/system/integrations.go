package system

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
)

// Integration kinds that POST /admin/integrations/test knows.
const (
	KindOnlyOffice = OfficeOnlyOffice
	KindEuroOffice = OfficeEuroOffice
	KindCollabora  = OfficeCollabora
	KindArrowLoop  = "arrowloop"
)

// What the token check found.
const (
	// TokenOK means the server took the secret or token.
	TokenOK = "ok"
	// TokenRejected means the server wants a different one.
	TokenRejected = "rejected"
	// TokenMissing means the server wants one and none was given.
	TokenMissing = "missing"
	// TokenOff means the server does not check, anyone may use it.
	TokenOff = "off"
	// TokenNone means the kind has no shared secret, as Collabora.
	TokenNone = "none"
)

// IntegrationTest asks to try an integration. Without Secret or Token the
// saved one is used when the address is the saved one.
type IntegrationTest struct {
	Kind   string  `json:"kind"`
	URL    string  `json:"url"`
	Secret *string `json:"secret,omitempty"`
	Token  *string `json:"token,omitempty"`
}

// IntegrationResult is what the test found. Error says in English what
// went wrong, for the log and the details.
type IntegrationResult struct {
	Kind      string `json:"kind"`
	OK        bool   `json:"ok"`
	Reachable bool   `json:"reachable"`
	Token     string `json:"token,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

// tester talks to the integrations. It only asks for their version and
// state, so no file and no user data leave the server.
type tester struct {
	client *http.Client
}

func newTester() tester {
	return tester{client: &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}}
}

func (t tester) run(ctx context.Context, kind, base, secret string) IntegrationResult {
	var res IntegrationResult
	switch kind {
	case KindOnlyOffice, KindEuroOffice:
		res = t.office(ctx, base, secret)
	case KindCollabora:
		res = t.collabora(ctx, base)
	case KindArrowLoop:
		res = t.arrowloop(ctx, base, secret)
	}
	res.Kind = kind
	res.OK = res.Reachable && res.Error == "" && (res.Token == TokenOK || res.Token == TokenOff || res.Token == TokenNone)
	return res
}

func (t tester) do(ctx context.Context, method, url string, body any, header http.Header) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "CannonCommander/"+version.Version)
	resp, err := t.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, err
}

// office checks an OnlyOffice or Euro-Office document server: the health
// check, then the version command once unsigned and, when the server wants
// a token, once signed with the secret.
func (t tester) office(ctx context.Context, base, secret string) IntegrationResult {
	var res IntegrationResult
	status, body, err := t.do(ctx, http.MethodGet, base+"/healthcheck", nil, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if status != http.StatusOK {
		res.Error = fmt.Sprintf("%s/healthcheck answered %d, this does not look like a document server", base, status)
		return res
	}
	res.Reachable = true
	if strings.TrimSpace(string(body)) != "true" {
		res.Error = "the document server reports that it is not healthy"
		return res
	}
	code, ver, err := t.command(ctx, base, "")
	if err != nil {
		res.Error = err.Error()
		return res
	}
	switch code {
	case 0:
		res.Token, res.Version = TokenOff, ver
		return res
	case officeBadToken:
		if secret == "" {
			res.Token = TokenMissing
			return res
		}
	default:
		res.Error = fmt.Sprintf("the command service answered with error %d", code)
		return res
	}
	code, ver, err = t.command(ctx, base, secret)
	switch {
	case err != nil:
		res.Error = err.Error()
	case code == 0:
		res.Token, res.Version = TokenOK, ver
	case code == officeBadToken:
		res.Token = TokenRejected
	default:
		res.Error = fmt.Sprintf("the command service answered with error %d", code)
	}
	return res
}

// officeBadToken is the command service's error for a missing or wrong JWT.
const officeBadToken = 6

// command sends the version command. The token goes into the header and
// the body, because which one the server reads is its own setting.
func (t tester) command(ctx context.Context, base, secret string) (code int, ver string, err error) {
	body := map[string]any{"c": "version"}
	header := http.Header{}
	if secret != "" {
		inBody, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"c": "version"}).SignedString([]byte(secret))
		if err != nil {
			return 0, "", err
		}
		inHeader, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"payload": map[string]any{"c": "version"}}).SignedString([]byte(secret))
		if err != nil {
			return 0, "", err
		}
		body["token"] = inBody
		header.Set("Authorization", "Bearer "+inHeader)
	}
	var data []byte
	status := http.StatusNotFound
	for _, p := range []string{"/coauthoring/CommandService.ashx", "/command"} {
		status, data, err = t.do(ctx, http.MethodPost, base+p, body, header)
		if err != nil {
			return 0, "", err
		}
		if status != http.StatusNotFound {
			break
		}
	}
	if status != http.StatusOK {
		return 0, "", fmt.Errorf("the command service answered %d", status)
	}
	var out struct {
		Error   int    `json:"error"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, "", fmt.Errorf("the command service answered something other than JSON")
	}
	return out.Error, out.Version, nil
}

// collabora checks the WOPI discovery. Collabora has no shared secret, it
// decides by its allow list which hosts may use it.
func (t tester) collabora(ctx context.Context, base string) IntegrationResult {
	res := IntegrationResult{Token: TokenNone}
	status, body, err := t.do(ctx, http.MethodGet, base+"/hosting/discovery", nil, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if status != http.StatusOK || !bytes.Contains(body, []byte("<wopi-discovery")) {
		res.Error = fmt.Sprintf("%s/hosting/discovery answered %d without a WOPI discovery, this does not look like Collabora", base, status)
		return res
	}
	res.Reachable = true
	if status, body, err := t.do(ctx, http.MethodGet, base+"/hosting/capabilities", nil, nil); err == nil && status == http.StatusOK {
		var caps struct {
			ProductVersion string `json:"productVersion"`
		}
		if json.Unmarshal(body, &caps) == nil {
			res.Version = caps.ProductVersion
		}
	}
	return res
}

// arrowloop checks that the address is ArrowLoop and that the token opens
// its job list, the one place the token is good for.
func (t tester) arrowloop(ctx context.Context, base, token string) IntegrationResult {
	var res IntegrationResult
	status, body, err := t.do(ctx, http.MethodGet, base+"/api/session", nil, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	var session struct {
		Required *bool `json:"required"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &session) != nil || session.Required == nil {
		res.Error = fmt.Sprintf("%s/api/session answered %d, this does not look like ArrowLoop", base, status)
		return res
	}
	res.Reachable = true
	if token == "" {
		res.Token = TokenMissing
		if !*session.Required {
			res.Token = TokenOff
		}
		return res
	}
	status, _, err = t.do(ctx, http.MethodGet, base+"/api/jobs", nil, http.Header{"X-Arrowloop-Token": {token}})
	switch {
	case err != nil:
		res.Error = err.Error()
	case status == http.StatusOK:
		res.Token = TokenOK
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		res.Token = TokenRejected
	default:
		res.Error = fmt.Sprintf("%s/api/jobs answered %d", base, status)
	}
	return res
}
