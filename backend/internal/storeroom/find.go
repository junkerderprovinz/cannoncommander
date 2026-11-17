// Package storeroom finds what a server no longer uses (container templates,
// appdata folders and Docker's leftovers) and keeps it in a storeroom, from
// where it can come back. Only images and the build cache, which can be
// downloaded or built again, are removed for good.
package storeroom

import (
	"cmp"
	"context"
	"encoding/xml"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Config says where the server keeps what the storeroom looks at.
type Config struct {
	// Templates is dockerMan's folder of user templates.
	Templates string
	// Plugins is searched for any mention of an appdata folder, in the
	// settings and compose files of every plugin.
	Plugins string
	// Appdata is the share the containers keep their data in.
	Appdata string
	// Room holds the storeroom's index and the templates in it. Folders
	// wait in a hidden folder on their own share, so they move by a rename.
	Room string
	// Docker is the engine's socket. Without it nothing counts as unused,
	// because only the engine knows what the containers use.
	Docker string
	// Write allows moving anything at all.
	Write bool
}

// quiet is how long a folder must have gone unchanged to count as orphaned.
const quiet = 30 * 24 * time.Hour

// Service scans and keeps the storeroom. It is safe for concurrent use.
type Service struct {
	cfg    Config
	docker *docker
	now    func() time.Time
	mu     sync.Mutex
}

// New returns a service for cfg.
func New(cfg Config) *Service {
	s := &Service{cfg: cfg, now: time.Now}
	if cfg.Docker != "" {
		s.docker = dialDocker(cfg.Docker)
	}
	return s
}

// Find is one thing nothing uses. ID tells it apart within its kind: a path,
// a container or network ID, a volume name or an image ID.
type Find struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Size int64  `json:"size"`
	// ForGood is set for what is removed instead of kept.
	ForGood bool `json:"forGood"`
}

// Scan is what the storeroom found and what it holds.
type Scan struct {
	Docker bool    `json:"docker"`
	Write  bool    `json:"write"`
	Finds  []Find  `json:"finds"`
	Room   []Entry `json:"room"`
}

// world is what the engine and the flash say at one moment.
type world struct {
	containers []container
	networks   []network
	volumes    []volume
	usage      usage
	templates  map[string]template
	mentions   string
	held       map[string]bool
}

type template struct {
	path, name, network, text string
}

// Scan looks at the server. When the engine does not answer it reports no
// finds at all, since a template or folder may belong to a container it
// cannot see.
func (s *Service) Scan(ctx context.Context) (*Scan, error) {
	room, err := s.load()
	if err != nil {
		return nil, err
	}
	out := &Scan{Write: s.cfg.Write, Finds: []Find{}, Room: room}
	w, err := s.look(ctx, room)
	if err != nil {
		return out, nil
	}
	out.Docker = true
	out.Finds = s.finds(w)
	return out, nil
}

func (s *Service) look(ctx context.Context, room []Entry) (*world, error) {
	if s.docker == nil {
		return nil, errNoDocker
	}
	w := &world{held: map[string]bool{}}
	var err error
	if w.containers, err = s.docker.containers(ctx); err != nil {
		return nil, err
	}
	if w.networks, err = s.docker.networks(ctx); err != nil {
		return nil, err
	}
	if w.volumes, err = s.docker.danglingVolumes(ctx); err != nil {
		return nil, err
	}
	if w.usage, err = s.docker.usage(ctx); err != nil {
		return nil, err
	}
	if w.templates, err = readTemplates(s.cfg.Templates); err != nil {
		return nil, err
	}
	w.mentions = readMentions(s.cfg.Plugins)
	for _, e := range room {
		w.held[e.Kind+"\n"+e.From] = true
		w.held[e.Kind+"\n"+e.Held] = true
		w.held[e.Kind+"\n"+e.Ref] = true
	}
	return w, nil
}

func readTemplates(dir string) (map[string]template, error) {
	out := map[string]template{}
	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var x struct {
			Name    string
			Network string
		}
		t := template{path: f, text: string(raw)}
		// A template that does not parse still names its folders in the text.
		if xml.Unmarshal(raw, &x) == nil {
			t.name, t.network = strings.TrimSpace(x.Name), strings.TrimSpace(x.Network)
		}
		out[f] = t
	}
	return out, nil
}

// readMentions gathers the small text files of every plugin, which is where
// compose stacks and plugin settings name the folders they use.
func readMentions(dir string) string {
	var b strings.Builder
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".cfg", ".yml", ".yaml", ".json", ".env", ".xml", ".conf", ".ini":
		default:
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > 1<<20 {
			return nil
		}
		if raw, err := os.ReadFile(p); err == nil {
			b.Write(raw)
			b.WriteByte('\n')
		}
		return nil
	})
	return b.String()
}

func (s *Service) finds(w *world) []Find {
	var out []Find
	out = append(out, s.unusedTemplates(w)...)
	out = append(out, s.orphanedFolders(w)...)
	out = append(out, s.leftoverContainers(w)...)
	out = append(out, unusedNetworks(w)...)
	out = append(out, leftoverVolumes(w)...)
	out = append(out, unusedImages(w)...)
	return out
}

func (w *world) containerNamed(name string) bool {
	return slices.ContainsFunc(w.containers, func(c container) bool { return c.name() == name })
}

func (s *Service) unusedTemplates(w *world) []Find {
	var out []Find
	for _, t := range w.templates {
		if t.name == "" || !strings.HasPrefix(filepath.Base(t.path), "my-") || w.containerNamed(t.name) || w.held["template\n"+t.path] {
			continue
		}
		out = append(out, Find{Kind: "template", ID: t.path, Name: t.name, Path: t.path, Size: size(t.path)})
	}
	return byName(out)
}

// usedFolder reports the appdata folder a host path lies in, as Unraid
// writes it for any pool or the user share: /mnt/<pool>/<share>/<folder>.
func usedFolder(share, host string) string {
	parts := strings.Split(strings.Trim(path.Clean(host), "/"), "/")
	if len(parts) < 4 || parts[0] != "mnt" || parts[2] != share {
		return ""
	}
	return parts[3]
}

func (s *Service) orphanedFolders(w *world) []Find {
	share := filepath.Base(s.cfg.Appdata)
	used := map[string]bool{}
	for _, c := range w.containers {
		for _, m := range c.Mounts {
			used[usedFolder(share, m.Source)] = true
		}
	}
	text := w.mentions
	for _, t := range w.templates {
		text += t.text
	}
	entries, err := os.ReadDir(s.cfg.Appdata)
	if err != nil {
		return nil
	}
	var out []Find
	for _, e := range entries {
		name := e.Name()
		dir := filepath.Join(s.cfg.Appdata, name)
		if !e.IsDir() || strings.HasPrefix(name, ".") || used[name] || mentioned(text, share, name) || w.held["appdata\n"+dir] || s.changedLately(dir) {
			continue
		}
		out = append(out, Find{Kind: "appdata", ID: dir, Name: name, Path: dir, Size: size(dir)})
	}
	return byName(out)
}

// mentioned reports whether text names the folder share/name as a path.
func mentioned(text, share, name string) bool {
	re := regexp.MustCompile(`/` + regexp.QuoteMeta(share) + `/` + regexp.QuoteMeta(name) + `([^A-Za-z0-9._-]|$)`)
	return re.MatchString(text)
}

// changedLately looks at the folder and what lies directly in it. A deeper
// walk would read the whole share.
func (s *Service) changedLately(dir string) bool {
	since := s.now().Add(-quiet)
	info, err := os.Stat(dir)
	if err != nil || info.ModTime().After(since) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.ModTime().After(since) {
			return true
		}
	}
	return false
}

// leftoverContainers are stopped containers that neither Unraid nor a
// compose stack nor a template manages, made more than quiet ago.
func (s *Service) leftoverContainers(w *world) []Find {
	names := map[string]bool{}
	for _, t := range w.templates {
		names[t.name] = true
	}
	since := s.now().Add(-quiet).Unix()
	var out []Find
	for _, c := range w.containers {
		if (c.State != "exited" && c.State != "created") || c.Created > since || names[c.name()] || w.held["container\n"+c.ID] || managed(c.Labels) {
			continue
		}
		out = append(out, Find{Kind: "container", ID: c.ID, Name: c.name()})
	}
	return byName(out)
}

func managed(labels map[string]string) bool {
	for k := range labels {
		if strings.HasPrefix(k, "net.unraid.docker.") || strings.HasPrefix(k, "com.docker.compose.") || strings.HasPrefix(k, "com.docker.stack.") {
			return true
		}
	}
	return false
}

// unusedNetworks are plain bridge networks of our own that no container,
// running or stopped, and no template names. Unraid's own networks use
// other drivers.
func unusedNetworks(w *world) []Find {
	used := map[string]bool{}
	for _, c := range w.containers {
		used[c.HostConfig.NetworkMode] = true
		for n := range c.NetworkSettings.Networks {
			used[n] = true
		}
	}
	for _, t := range w.templates {
		used[t.network] = true
	}
	var out []Find
	for _, n := range w.networks {
		if n.Driver != "bridge" || n.Scope != "local" || n.Name == "bridge" || len(n.Containers) > 0 || used[n.Name] || used[n.ID] || managed(n.Labels) || w.held["network\n"+n.Name] {
			continue
		}
		out = append(out, Find{Kind: "network", ID: n.ID, Name: n.Name})
	}
	return byName(out)
}

var anonymous = regexp.MustCompile(`^[0-9a-f]{64}$`)

// leftoverVolumes are anonymous volumes no container holds. A named one
// may wait for a container that comes back.
func leftoverVolumes(w *world) []Find {
	sizes := map[string]int64{}
	for _, v := range w.usage.Volumes {
		if v.UsageData != nil && v.UsageData.Size > 0 {
			sizes[v.Name] = v.UsageData.Size
		}
	}
	var out []Find
	for _, v := range w.volumes {
		if !anonymous.MatchString(v.Name) || managed(v.Labels) || w.held["volume\n"+v.Name] {
			continue
		}
		out = append(out, Find{Kind: "volume", ID: v.Name, Name: v.Name, Size: sizes[v.Name]})
	}
	return byName(out)
}

func unusedImages(w *world) []Find {
	var out []Find
	for _, im := range w.usage.Images {
		if im.Containers != 0 {
			continue
		}
		name := ""
		for _, tag := range im.RepoTags {
			if tag != "<none>:<none>" {
				name = tag
				break
			}
		}
		out = append(out, Find{Kind: "image", ID: im.ID, Name: name, Size: im.Size, ForGood: true})
	}
	slices.SortStableFunc(out, func(a, b Find) int { return cmp.Compare(b.Size, a.Size) })
	var cache int64
	for _, c := range w.usage.BuildCache {
		if !c.InUse {
			cache += c.Size
		}
	}
	if cache > 0 {
		out = append(out, Find{Kind: "cache", ID: "build-cache", Size: cache, ForGood: true})
	}
	return out
}

func byName(f []Find) []Find {
	slices.SortFunc(f, func(a, b Find) int { return strings.Compare(a.Name, b.Name) })
	return f
}

// size adds up the files below p.
func size(p string) int64 {
	var n int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
