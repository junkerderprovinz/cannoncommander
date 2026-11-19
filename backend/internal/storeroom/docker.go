package storeroom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// docker speaks the few calls of the Engine API the storeroom needs. Paths
// carry no API version, so the engine answers in its own.
type docker struct {
	c    *http.Client
	base string
}

func dialDocker(socket string) *docker {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", socket)
	}}
	return &docker{c: &http.Client{Transport: tr, Timeout: 30 * time.Second}, base: "http://docker"}
}

// errNotFound is the engine's 404, a container or network that went away.
var errNotFound = errors.New("storeroom: not found in docker")

func (d *docker) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, d.base+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := d.c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if res.StatusCode >= 300 {
		var msg struct{ Message string }
		_ = json.NewDecoder(res.Body).Decode(&msg)
		return fmt.Errorf("docker %s %s: %d %s", method, path, res.StatusCode, msg.Message)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type mount struct {
	Type   string
	Name   string
	Source string
}

type container struct {
	ID         string `json:"Id"`
	Names      []string
	Image      string
	ImageID    string
	State      string
	Created    int64
	Labels     map[string]string
	Mounts     []mount
	HostConfig struct {
		NetworkMode string
	}
	NetworkSettings struct {
		Networks map[string]json.RawMessage
	}
}

// name is the container's name without the slash the engine puts in front.
func (c container) name() string {
	if len(c.Names) == 0 {
		return ""
	}
	n := c.Names[0]
	if len(n) > 0 && n[0] == '/' {
		n = n[1:]
	}
	return n
}

type network struct {
	ID         string `json:"Id"`
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Attachable bool
	EnableIPv6 bool
	IPAM       json.RawMessage
	Options    map[string]string
	Labels     map[string]string
	Containers map[string]json.RawMessage
}

type volume struct {
	Name      string
	Driver    string
	Labels    map[string]string
	UsageData *struct{ Size int64 }
}

type image struct {
	ID          string `json:"Id"`
	RepoTags    []string
	RepoDigests []string
	Size        int64
	// Containers counts running and stopped ones alike, -1 when the engine
	// did not count.
	Containers int64
}

// tags are the image's names, without the placeholder of one that has none.
func (im image) tags() []string {
	var out []string
	for _, t := range im.RepoTags {
		if t != "<none>:<none>" {
			out = append(out, t)
		}
	}
	return out
}

// manifest is one entry of an image's index in containerd's store. Available
// is false for what a pull left at the registry, such as other platforms.
type manifest struct {
	Kind      string
	Available bool
}

type usage struct {
	Images     []image
	Volumes    []volume
	BuildCache []struct {
		ID    string
		Size  int64
		InUse bool
	}
}

func (d *docker) containers(ctx context.Context) ([]container, error) {
	var out []container
	return out, d.do(ctx, http.MethodGet, "/containers/json?all=1", nil, &out)
}

func (d *docker) networks(ctx context.Context) ([]network, error) {
	var out []network
	return out, d.do(ctx, http.MethodGet, "/networks", nil, &out)
}

func (d *docker) network(ctx context.Context, id string) (network, error) {
	var out network
	return out, d.do(ctx, http.MethodGet, "/networks/"+url.PathEscape(id), nil, &out)
}

func (d *docker) danglingVolumes(ctx context.Context) ([]volume, error) {
	var out struct{ Volumes []volume }
	q := url.Values{"filters": {`{"dangling":["true"]}`}}
	return out.Volumes, d.do(ctx, http.MethodGet, "/volumes?"+q.Encode(), nil, &out)
}

func (d *docker) usage(ctx context.Context) (usage, error) {
	var out usage
	return out, d.do(ctx, http.MethodGet, "/system/df", nil, &out)
}

// containerdStore reports whether the engine keeps its images in containerd,
// which gives every image a digest, one built here as well.
func (d *docker) containerdStore(ctx context.Context) (bool, error) {
	var out struct{ DriverStatus [][2]string }
	if err := d.do(ctx, http.MethodGet, "/info", nil, &out); err != nil {
		return false, err
	}
	for _, kv := range out.DriverStatus {
		if kv[0] == "driver-type" && kv[1] == "io.containerd.snapshotter.v1" {
			return true, nil
		}
	}
	return false, nil
}

func (d *docker) manifests(ctx context.Context, id string) ([]manifest, error) {
	var out struct{ Manifests []manifest }
	return out.Manifests, d.do(ctx, http.MethodGet, "/images/"+url.PathEscape(id)+"/json?manifests=1", nil, &out)
}

func (d *docker) rename(ctx context.Context, id, name string) error {
	return d.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/rename?"+url.Values{"name": {name}}.Encode(), nil, nil)
}

func (d *docker) removeContainer(ctx context.Context, id string) error {
	return d.do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id), nil, nil)
}

func (d *docker) removeNetwork(ctx context.Context, id string) error {
	return d.do(ctx, http.MethodDelete, "/networks/"+url.PathEscape(id), nil, nil)
}

func (d *docker) createNetwork(ctx context.Context, body json.RawMessage) error {
	return d.do(ctx, http.MethodPost, "/networks/create", body, nil)
}

func (d *docker) removeVolume(ctx context.Context, name string) error {
	return d.do(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), nil, nil)
}

func (d *docker) removeImage(ctx context.Context, id string) error {
	return d.do(ctx, http.MethodDelete, "/images/"+url.PathEscape(id), nil, nil)
}

// pruneBuildCache drops the build cache no build uses. The engine keeps
// what a running build holds.
func (d *docker) pruneBuildCache(ctx context.Context) error {
	return d.do(ctx, http.MethodPost, "/build/prune", nil, nil)
}
