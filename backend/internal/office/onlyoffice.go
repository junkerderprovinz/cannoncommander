package office

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// documentTypes are the editors of the document server by file extension.
var documentTypes = map[string]string{}

func init() {
	for kind, exts := range map[string]string{
		"word":  "doc docm docx dot dotm dotx epub fb2 fodt htm html mht mhtml odt ott rtf txt xml",
		"cell":  "csv et ett fods ods ots xls xlsb xlsm xlsx xlt xltm xltx",
		"slide": "dps dpt fodp odp otp pot potm potx pps ppsm ppsx ppt pptm pptx",
		"pdf":   "djvu oxps pdf xps",
	} {
		for _, e := range strings.Fields(exts) {
			documentTypes[e] = kind
		}
	}
}

func (s *service) editorConfig(r *http.Request, o system.Office, u *users.User, loc ccfs.Location, info os.FileInfo, g grant, token string) (map[string]any, error) {
	e := ext(loc.Path)
	kind, ok := documentTypes[e]
	if !ok {
		return nil, v1.Errorf(http.StatusBadRequest, "the office server does not open ."+e+" files")
	}
	base := s.base(r)
	mode := "view"
	if g.Write {
		mode = "edit"
	}
	cfg := map[string]any{
		"documentType": kind,
		"document": map[string]any{
			"fileType": e,
			"key":      documentKey(loc, info),
			"title":    path.Base(loc.Path),
			"url":      base + "/office/file?token=" + url.QueryEscape(token),
			"permissions": map[string]any{
				"edit":     g.Write,
				"download": true,
				"print":    true,
			},
		},
		"editorConfig": map[string]any{
			"mode":        mode,
			"callbackUrl": base + "/office/callback?token=" + url.QueryEscape(token),
			"lang":        officeLang(u),
			"user":        map[string]any{"id": strconv.FormatUint(u.ID, 10), "name": u.Username},
		},
	}
	if o.Secret != "" {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(cfg)).SignedString([]byte(o.Secret))
		if err != nil {
			return nil, err
		}
		cfg["token"] = signed
	}
	return cfg, nil
}

// onlyofficeFile hands the document to the document server, which fetches
// it itself instead of the browser.
func (s *service) onlyofficeFile(w http.ResponseWriter, r *http.Request) error {
	return s.serveFile(w, r, r.URL.Query().Get("token"))
}

func (s *service) serveFile(w http.ResponseWriter, r *http.Request, token string) error {
	_, _, loc, info, err := s.granted(r.Context(), token, false)
	if err != nil {
		return publicError(err)
	}
	f, err := os.Open(loc.Real)
	if err != nil {
		return publicError(err)
	}
	defer f.Close()
	// These routes are public and only a server reads them, so nothing here
	// has to open in a browser, where a page would run on the app's origin.
	hdr := w.Header()
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("Content-Type", "application/octet-stream")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", fsapi.SandboxPolicy)
	hdr.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(loc.Path)}))
	http.ServeContent(w, r, path.Base(loc.Path), info.ModTime(), f)
	return nil
}

// callback is what the document server reports about an open document.
type callback struct {
	Key    string `json:"key"`
	Status int    `json:"status"`
	URL    string `json:"url"`
	Token  string `json:"token"`
}

// The statuses that carry a document to save: all editors closed, and a
// save forced from the editor.
const (
	statusReady     = 2
	statusForceSave = 6
)

// onlyofficeCallback takes the saved document. The document server wants
// {"error":0} for every callback it may forget about.
func (s *service) onlyofficeCallback(w http.ResponseWriter, r *http.Request) error {
	answer := func(code int) error { return v1.WriteJSON(w, r, http.StatusOK, map[string]int{"error": code}) }
	o, err := s.settings(r.Context())
	if err != nil {
		return err
	}
	cb, err := readCallback(r, o.Secret)
	if err != nil {
		logger.Errorf("office callback: %v", err)
		return answer(1)
	}
	if cb.Status != statusReady && cb.Status != statusForceSave {
		return answer(0)
	}
	_, _, loc, info, err := s.granted(r.Context(), r.URL.Query().Get("token"), true)
	if err != nil {
		logger.Errorf("office callback: %v", err)
		return answer(1)
	}
	if err := s.fetchInto(r, o.URL, cb.URL, loc, info); err != nil {
		logger.Errorf("office callback for %s: %v", loc.Path, err)
		return answer(1)
	}
	return answer(0)
}

// readCallback decodes the callback. With a secret only a signed one
// counts, from the body or from the Authorization header, where the
// document server wraps it in payload.
func readCallback(r *http.Request, secret string) (callback, error) {
	var cb callback
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&cb); err != nil {
		return cb, err
	}
	if secret == "" {
		return cb, nil
	}
	raw, wrapped := cb.Token, false
	if raw == "" {
		raw, wrapped = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), true
	}
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return callback{}, fmt.Errorf("callback not signed with the JWT secret: %w", err)
	}
	payload := map[string]any(claims)
	if p, ok := claims["payload"].(map[string]any); ok && wrapped {
		payload = p
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return callback{}, err
	}
	cb = callback{}
	return cb, json.Unmarshal(data, &cb)
}

// fetchInto downloads the saved document from the document server. Only
// an address on that server is fetched, so a forged callback cannot make
// this server load anything else.
func (s *service) fetchInto(r *http.Request, server, raw string, loc ccfs.Location, info os.FileInfo) error {
	got, err := url.Parse(raw)
	if err != nil {
		return err
	}
	want, err := url.Parse(server)
	if err != nil {
		return err
	}
	if !sameOrigin(want, got) {
		return errors.New("the saved document is not on the document server: " + got.Scheme + "://" + got.Host)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, got.String(), nil)
	if err != nil {
		return err
	}
	client := *s.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("the document server redirects too often")
		}
		if !sameOrigin(want, next.URL) {
			return errors.New("the document server redirects elsewhere: " + next.URL.Scheme + "://" + next.URL.Host)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the document server answered %s", resp.Status)
	}
	return save(loc, info, func(f *os.File) error {
		_, err := io.Copy(f, resp.Body)
		return err
	})
}

// publicError keeps what a caller without a session learns short.
func publicError(err error) error {
	var p *v1.Problem
	switch {
	case errors.Is(err, errBadToken):
		return v1.Errorf(http.StatusUnauthorized, errBadToken.Error())
	case errors.As(err, &p):
		return p
	}
	return v1.ProblemFor(err)
}

func officeLang(u *users.User) string {
	if u.Locale != "" {
		return u.Locale
	}
	return "en"
}

// sameOrigin reports whether two addresses lead to the same scheme, host and
// port, with a missing port taken as the default of the scheme.
func sameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && originPort(a) == originPort(b)
}

func originPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
