package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
)

// KindFetch loads a file from a web address into a folder.
const KindFetch = "fetchUrl"

func init() {
	jobs.RegisterKind(KindFetch, jobs.Kind{Prepare: prepareFetch, Run: runFetch, Heavy: true})
}

// FetchParams are the params of fetchUrl. Name defaults to the last part
// of the address.
type FetchParams struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
	Dest string `json:"dest"`
}

// errPrivate is the answer for an address that leads into the server's
// own network, which a user could otherwise use to read what only the
// server can reach, such as a router's page or a container's API.
var errPrivate = errors.New("addresses in the local network or on the server itself cannot be loaded")

// globalV6 is the only part of the IPv6 space handed out for use on the
// internet. Outside it lie the forms that carry an IPv4 address, such as
// ::7f00:1 and 64:ff9b:1::/48, and retired ones such as fec0::/10.
var globalV6 = netip.MustParsePrefix("2000::/3")

// blocked are the ranges a fetch may not reach. Loopback, private,
// link local and the other special ranges come from netip, these are the
// ones it does not know. Teredo and 6to4 can tunnel to an IPv4 address of the
// sender's choosing, and nothing a download needs lives there.
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

// PublicAddr reports whether a may be fetched from, which only a globally
// routable address may.
func PublicAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return false
	}
	if a.Is6() && !globalV6.Contains(a) {
		return false
	}
	for _, p := range blocked {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// guard runs on every connection after the name is resolved, so a name
// that points inside, a redirect there or a DNS answer that changes in
// between are all caught.
func guard(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	a, err := netip.ParseAddr(host)
	if err != nil || !PublicAddr(a) {
		return errPrivate
	}
	return nil
}

// fetchClient follows up to five redirects and goes through no proxy, as
// a proxy would make the connection check meaningless.
var fetchClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, Control: guard}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("the address redirects too often")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return errors.New("the address redirects to something other than http or https")
		}
		return nil
	},
}

func parseFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return nil, badRequest("this is not a web address")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, badRequest("only http and https addresses can be loaded")
	}
	if u.User != nil {
		return nil, badRequest("an address with a user name or password cannot be loaded")
	}
	if a, err := netip.ParseAddr(strings.Trim(u.Hostname(), "[]")); err == nil && !PublicAddr(a) {
		return nil, v1.Errorf(http.StatusBadRequest, errPrivate.Error())
	}
	return u, nil
}

// NameFromURL is the file name an address suggests.
func NameFromURL(u *url.URL) string {
	name := path.Base(u.Path)
	if name == "/" || name == "." {
		return u.Hostname()
	}
	return name
}

func prepareFetch(_ context.Context, d v1.Deps, u *users.User, raw json.RawMessage) (*jobs.Plan, error) {
	var p FetchParams
	if err := jobs.Decode(raw, &p); err != nil {
		return nil, err
	}
	addr, err := parseFetchURL(p.URL)
	if err != nil {
		return nil, err
	}
	if p.Name == "" {
		p.Name = NameFromURL(addr)
	}
	p.Name = strings.TrimSpace(p.Name)
	if !ccfs.ValidName(p.Name) {
		return nil, badRequest("the file name is missing or holds a slash")
	}
	dest, err := folder(d.FS, u, p.Dest, ccfs.Create)
	if err != nil {
		return nil, err
	}
	p.URL, p.Dest = addr.String(), dest.Path
	return &jobs.Plan{Params: p, Name: p.Name, Dest: dest.Path,
		Items: []jobs.Item{{Seq: 0, Path: path.Join(dest.Path, p.Name), Name: p.Name}}}, nil
}

func runFetch(ctx context.Context, run *jobs.Run) error {
	var p FetchParams
	if err := run.Params(&p); err != nil {
		return err
	}
	it := run.Items()[0]
	if it.State == jobs.ItemDone {
		return nil
	}
	dest, err := run.Deps().FS.Resolve(p.Dest)
	if err != nil {
		return err
	}
	run.Begin(it.Seq)
	run.Current(p.Name)
	out, err := fetch(ctx, run, p, dest)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	run.Finish(it.Seq, jobs.ItemDone, err)
	if err != nil {
		return err
	}
	fschange.Created(out)
	run.SetResult(map[string]string{"path": out})
	return nil
}

func fetch(ctx context.Context, run *jobs.Run, p FetchParams, dest ccfs.Location) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "CannonCommander/"+version.Version)
	resp, err := fetchClient.Do(req)
	if err != nil {
		if errors.Is(err, errPrivate) {
			return "", errPrivate
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the server answered %s", resp.Status)
	}
	if resp.ContentLength > 0 {
		run.Measure(0, resp.ContentLength, 1)
	}
	name := p.Name
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && name == NameFromURL(req.URL) {
		if n := path.Base(params["filename"]); ccfs.ValidName(n) {
			name = n
		}
	}
	part := filepath.Join(dest.Real, "."+name+partSuffix)
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return "", err
	}
	_, err = copyCounted(ctx, run, f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		name = ccfs.FreeName(dest.Real, name)
		err = os.Rename(part, filepath.Join(dest.Real, name))
	}
	if err != nil {
		os.Remove(part)
		return "", err
	}
	if t, err := http.ParseTime(resp.Header.Get("Last-Modified")); err == nil {
		_ = os.Chtimes(filepath.Join(dest.Real, name), t, t)
	}
	return path.Join(p.Dest, name), nil
}
