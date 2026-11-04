package office

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// fileID is the WOPI id of a file, its path in a form a URL carries.
func fileID(p string) string { return base64.RawURLEncoding.EncodeToString([]byte(p)) }

// discovery reads which editor address Collabora offers per file type. Its
// answer only changes with Collabora's version, so it is kept for an hour.
type discovery struct {
	mu      sync.Mutex
	server  string
	at      time.Time
	actions map[string]string
}

type wopiDiscovery struct {
	Zones []struct {
		Apps []struct {
			Actions []struct {
				Name   string `xml:"name,attr"`
				Ext    string `xml:"ext,attr"`
				URLSrc string `xml:"urlsrc,attr"`
			} `xml:"action"`
		} `xml:"app"`
	} `xml:"net-zone"`
}

// placeholders are the optional parameters in an urlsrc, such as <ui=UI_LLCC&>.
var placeholders = regexp.MustCompile(`<[^>]*>`)

// action returns the editor address for a file type, ready for WOPISrc to
// be added.
func (d *discovery) action(ctx context.Context, client *http.Client, server, ext string, edit bool) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.server != server || time.Since(d.at) > time.Hour {
		actions, err := fetchDiscovery(ctx, client, server)
		if err != nil {
			return "", v1.Errorf(http.StatusBadGateway, "Collabora did not answer: "+err.Error())
		}
		d.server, d.at, d.actions = server, time.Now(), actions
	}
	names := []string{"view"}
	if edit {
		names = []string{"edit", "view"}
	}
	for _, n := range names {
		if src, ok := d.actions[n+" "+ext]; ok {
			src = placeholders.ReplaceAllString(src, "")
			if !strings.HasSuffix(src, "?") && !strings.HasSuffix(src, "&") {
				if strings.Contains(src, "?") {
					src += "&"
				} else {
					src += "?"
				}
			}
			return src, nil
		}
	}
	return "", v1.Errorf(http.StatusBadRequest, "Collabora does not open ."+ext+" files")
}

func fetchDiscovery(ctx context.Context, client *http.Client, server string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(server, "/")+"/hosting/discovery", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery answered %s", resp.Status)
	}
	var doc wopiDiscovery
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	actions := map[string]string{}
	for _, z := range doc.Zones {
		for _, a := range z.Apps {
			for _, act := range a.Actions {
				if act.Ext != "" {
					actions[act.Name+" "+act.Ext] = act.URLSrc
				}
			}
		}
	}
	return actions, nil
}

// lockTTL is how long a WOPI lock holds without a refresh, as the protocol
// says.
const lockTTL = 30 * time.Minute

type lock struct {
	id      string
	expires time.Time
}

// locks keeps the WOPI locks in memory. After a restart the editor locks
// again when it next saves.
type locks struct {
	mu sync.Mutex
	by map[string]lock
}

func newLocks() *locks { return &locks{by: map[string]lock{}} }

func (l *locks) current(p string) string {
	if lk, ok := l.by[p]; ok && time.Now().Before(lk.expires) {
		return lk.id
	}
	delete(l.by, p)
	return ""
}

// errLocked carries the lock that is in the way.
type errLocked struct{ held string }

func (e errLocked) Error() string { return "locked" }

// apply runs one lock operation of the WOPI protocol.
func (l *locks) apply(p, op, id, old string) (held string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cur := l.current(p)
	set := func() { l.by[p] = lock{id: id, expires: time.Now().Add(lockTTL)} }
	switch op {
	case "GET_LOCK":
		return cur, nil
	case "LOCK":
		switch {
		case old != "" && cur == old, old == "" && (cur == "" || cur == id):
			set()
			return id, nil
		}
	case "REFRESH_LOCK":
		if cur == id {
			set()
			return id, nil
		}
	case "UNLOCK":
		if cur == id {
			delete(l.by, p)
			return "", nil
		}
	default:
		return "", fmt.Errorf("unknown lock operation %q", op)
	}
	return cur, errLocked{cur}
}

// mayWrite reports whether a save with lock id may go ahead. An unlocked
// file takes any save, a locked one only the save of its holder.
func (l *locks) mayWrite(p, id string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cur := l.current(p)
	return cur, cur == "" || cur == id
}

func (s *service) wopiGrant(r *http.Request, write bool) (grant, string, error) {
	g, _, loc, _, err := s.granted(r.Context(), r.URL.Query().Get("access_token"), write)
	if err != nil {
		return grant{}, "", err
	}
	if fileID(loc.Path) != r.PathValue("id") {
		return grant{}, "", errBadToken
	}
	return g, loc.Path, nil
}

// checkFileInfo is the WOPI call that opens a document.
func (s *service) checkFileInfo(w http.ResponseWriter, r *http.Request) error {
	g, u, loc, info, err := s.granted(r.Context(), r.URL.Query().Get("access_token"), false)
	if err != nil {
		return publicError(err)
	}
	if fileID(loc.Path) != r.PathValue("id") {
		return publicError(errBadToken)
	}
	_, werr := s.fs.Check(u, loc.Path, ccfs.Modify)
	user := strconv.FormatUint(u.ID, 10)
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{
		"BaseFileName":            path.Base(loc.Path),
		"Size":                    info.Size(),
		"Version":                 version(info),
		"OwnerId":                 user,
		"UserId":                  user,
		"UserFriendlyName":        u.Username,
		"UserCanWrite":            g.Write && werr == nil,
		"UserCanNotWriteRelative": true,
		"SupportsLocks":           true,
		"SupportsGetLock":         true,
		"SupportsUpdate":          true,
		"LastModifiedTime":        info.ModTime().UTC().Format(time.RFC3339Nano),
	})
}

func (s *service) getFile(w http.ResponseWriter, r *http.Request) error {
	if _, _, err := s.wopiGrant(r, false); err != nil {
		return publicError(err)
	}
	return s.serveFile(w, r, r.URL.Query().Get("access_token"))
}

// lock answers the lock operations Collabora sends to the file URL.
func (s *service) lock(w http.ResponseWriter, r *http.Request) error {
	op := r.Header.Get("X-WOPI-Override")
	_, p, err := s.wopiGrant(r, op != "GET_LOCK")
	if err != nil {
		return publicError(err)
	}
	held, err := s.locks.apply(p, op, r.Header.Get("X-WOPI-Lock"), r.Header.Get("X-WOPI-OldLock"))
	w.Header().Set("X-WOPI-Lock", held)
	var locked errLocked
	switch {
	case errors.As(err, &locked):
		w.WriteHeader(http.StatusConflict)
	case err != nil:
		w.WriteHeader(http.StatusNotImplemented)
	default:
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

// putFile saves the document. Collabora sends the time of the version it
// started from, and a file changed meanwhile by someone else is refused, so
// the editor asks which one to keep.
func (s *service) putFile(w http.ResponseWriter, r *http.Request) error {
	if op := r.Header.Get("X-WOPI-Override"); op != "" && op != "PUT" {
		return v1.Errorf(http.StatusNotImplemented, op+" is not supported")
	}
	_, _, loc, info, err := s.granted(r.Context(), r.URL.Query().Get("access_token"), true)
	if err != nil {
		return publicError(err)
	}
	if fileID(loc.Path) != r.PathValue("id") {
		return publicError(errBadToken)
	}
	if held, ok := s.locks.mayWrite(loc.Path, r.Header.Get("X-WOPI-Lock")); !ok {
		w.Header().Set("X-WOPI-Lock", held)
		w.WriteHeader(http.StatusConflict)
		return nil
	}
	if ts := r.Header.Get("X-COOL-WOPI-Timestamp"); ts != "" {
		if was, err := time.Parse(time.RFC3339Nano, ts); err == nil && !was.Equal(info.ModTime().UTC()) {
			return v1.WriteJSON(w, r, http.StatusConflict, map[string]int{"COOLStatusCode": 1010})
		}
	}
	err = save(loc, info, func(f *os.File) error {
		_, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxDocument))
		return err
	})
	if err != nil {
		return publicError(err)
	}
	after, err := os.Stat(loc.Real)
	if err != nil {
		return publicError(err)
	}
	w.Header().Set("X-WOPI-ItemVersion", version(after))
	return v1.WriteJSON(w, r, http.StatusOK, map[string]string{"LastModifiedTime": after.ModTime().UTC().Format(time.RFC3339Nano)})
}

// maxDocument caps what a save may write, far above any office document.
const maxDocument = 1 << 30
