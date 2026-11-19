// Package storeroom finds what a server no longer uses (container templates,
// compose stacks, appdata folders and Docker's leftovers) and keeps it in a
// storeroom, from where it can come back. Only images and the build cache,
// which can be downloaded or built again, are removed for good.
package storeroom

import (
	"cmp"
	"context"
	"encoding/xml"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Config says where the server keeps what the storeroom looks at.
type Config struct {
	// Templates is dockerMan's folder of user templates.
	Templates string
	// Stacks is where the compose plugin keeps a folder for each stack.
	Stacks string
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
	stacks     []stack
	mentions   string
	// compose is the text of every stack, stored ones included, where a
	// folder may hide behind a variable.
	compose string
	room    []Entry
	held    map[string]bool
	// pulled holds the images a registry sent, where a digest does not tell:
	// in containerd's store only a pull leaves part of an index at the
	// registry. It is nil for the classic store.
	pulled map[string]bool
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
	w := &world{room: room, held: map[string]bool{}}
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
	if w.pulled, err = s.pulled(ctx, w.usage.Images); err != nil {
		return nil, err
	}
	if w.templates, err = readTemplates(s.cfg.Templates); err != nil {
		return nil, err
	}
	w.stacks = readStacks(s.cfg.Stacks)
	w.mentions = readMentions(s.cfg.Plugins)
	for _, st := range w.stacks {
		w.compose += st.text
	}
	// What waits in the storeroom may come back with the folders it names.
	for _, e := range room {
		w.held[e.Kind+"\n"+e.From] = true
		w.held[e.Kind+"\n"+e.Held] = true
		w.held[e.Kind+"\n"+e.Ref] = true
		switch e.Kind {
		case "template":
			if raw, err := os.ReadFile(e.Held); err == nil {
				w.mentions += string(raw) + "\n"
			}
		case "stack":
			if st, ok := readStack(e.Held); ok {
				w.compose += st.text
			}
		}
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
		if err == nil && !d.IsDir() {
			readText(&b, p, d)
		}
		return nil
	})
	return b.String()
}

// readText adds p to b when it is a small file of the kinds that name folders.
func readText(b *strings.Builder, p string, d fs.DirEntry) {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".cfg", ".yml", ".yaml", ".json", ".env", ".xml", ".conf", ".ini":
	default:
		return
	}
	if info, err := d.Info(); err != nil || info.Size() > 1<<20 {
		return
	}
	if raw, err := os.ReadFile(p); err == nil {
		b.Write(raw)
		b.WriteByte('\n')
	}
}

// stack is a compose stack as the compose plugin keeps it: a folder named
// after the stack, with the compose file in it or, for a stack kept
// elsewhere, a file named indirect that holds the folder it lies in.
type stack struct {
	dir, name string
	// places are the folders its compose files lie in, dir first.
	places []string
	text   string
	// containers are the names its compose files give their containers.
	containers []string
}

var containerName = regexp.MustCompile(`(?m)^\s*container_name:\s*["']?([^"'\s#]+)`)

func readStacks(dir string) []stack {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []stack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if st, ok := readStack(filepath.Join(dir, e.Name())); ok {
			out = append(out, st)
		}
	}
	return out
}

// readStack reports false when dir has no compose file and points nowhere else.
func readStack(dir string) (stack, bool) {
	st := stack{dir: dir, name: filepath.Base(dir), places: []string{dir}}
	if raw, err := os.ReadFile(filepath.Join(dir, "name")); err == nil && strings.TrimSpace(string(raw)) != "" {
		st.name = strings.TrimSpace(string(raw))
	}
	ok := false
	if raw, err := os.ReadFile(filepath.Join(dir, "indirect")); err == nil && strings.TrimSpace(string(raw)) != "" {
		st.places, ok = append(st.places, filepath.Clean(strings.TrimSpace(string(raw)))), true
	}
	var b strings.Builder
	for _, place := range st.places {
		entries, _ := os.ReadDir(place)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			switch e.Name() {
			case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
				ok = true
			}
			readText(&b, filepath.Join(place, e.Name()), e)
		}
	}
	st.text = b.String()
	for _, m := range containerName.FindAllStringSubmatch(st.text, -1) {
		st.containers = append(st.containers, m[1])
	}
	return st, ok
}

// loose is a project name the way compose and the plugin may have bent it:
// lower case, without the spaces, dots and dashes they replace.
func loose(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, name)
}

// within reports whether p is dir or lies below it.
func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// up reports whether any container, running or stopped, belongs to st: by
// its project name, by the folder its compose files came from, or by a
// container name the compose file sets.
func (w *world) up(st stack) bool {
	for _, c := range w.containers {
		if slices.Contains(st.containers, c.name()) {
			return true
		}
		if p := loose(c.Labels["com.docker.compose.project"]); p != "" && (p == loose(filepath.Base(st.dir)) || p == loose(st.name)) {
			return true
		}
		from := append(strings.Split(c.Labels["com.docker.compose.project.config_files"], ","), c.Labels["com.docker.compose.project.working_dir"])
		for _, f := range from {
			if f != "" && slices.ContainsFunc(st.places, func(place string) bool { return within(place, f) }) {
				return true
			}
		}
	}
	return false
}

func (s *Service) finds(w *world) []Find {
	var out []Find
	out = append(out, s.unusedTemplates(w)...)
	out = append(out, s.orphanedStacks(w)...)
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

// unusedTemplates are the user templates no container is named after,
// neither by the name inside nor by the file's name.
func (s *Service) unusedTemplates(w *world) []Find {
	var out []Find
	for _, t := range w.templates {
		file := filepath.Base(t.path)
		if t.name == "" || !strings.HasPrefix(file, "my-") || w.containerNamed(t.name) || w.containerNamed(strings.TrimSuffix(file[3:], ".xml")) || w.held["template\n"+t.path] {
			continue
		}
		out = append(out, Find{Kind: "template", ID: t.path, Name: t.name, Path: t.path, Size: size(t.path)})
	}
	return byName(out)
}

// orphanedStacks are stacks no container belongs to, left unchanged for
// quiet. A folder elsewhere that is gone counts as unchanged.
func (s *Service) orphanedStacks(w *world) []Find {
	var out []Find
	for _, st := range w.stacks {
		if w.up(st) || w.held["stack\n"+st.dir] || slices.ContainsFunc(st.places, func(p string) bool { return exists(p) && s.changedLately(p) }) {
			continue
		}
		out = append(out, Find{Kind: "stack", ID: st.dir, Name: st.name, Path: st.dir, Size: size(st.dir)})
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

// orphanedFolders are appdata folders nothing mounts or names, left
// unchanged for quiet. A folder named like a container, a template or a
// stack stays as well, since it most likely belongs to it.
func (s *Service) orphanedFolders(w *world) []Find {
	share := filepath.Base(s.cfg.Appdata)
	used := map[string]bool{}
	for _, c := range w.containers {
		used[strings.ToLower(c.name())] = true
		for _, m := range c.Mounts {
			used[usedFolder(share, m.Source)] = true
		}
	}
	text := w.mentions
	for _, t := range w.templates {
		used[strings.ToLower(t.name)] = true
		text += t.text
	}
	for _, st := range w.stacks {
		used[strings.ToLower(st.name)], used[strings.ToLower(filepath.Base(st.dir))] = true, true
	}
	for _, e := range w.room {
		if e.Kind == "container" || e.Kind == "template" || e.Kind == "stack" {
			used[strings.ToLower(e.Name)] = true
		}
	}
	entries, err := os.ReadDir(s.cfg.Appdata)
	if err != nil {
		return nil
	}
	var out []Find
	for _, e := range entries {
		name := e.Name()
		dir := filepath.Join(s.cfg.Appdata, name)
		if !e.IsDir() || strings.HasPrefix(name, ".") || used[name] || used[strings.ToLower(name)] || mentioned(text, share, name) || named(w.compose, name) || w.held["appdata\n"+dir] || s.changedLately(dir) {
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

// named reports whether a compose text has a path that ends in the folder's
// name, as ${APPDATA}/name does.
func named(text, name string) bool {
	re := regexp.MustCompile(`/` + regexp.QuoteMeta(name) + `([^A-Za-z0-9._-]|$)`)
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

// pulled asks containerd's store which unused images came from a registry,
// and returns nil for the classic store.
func (s *Service) pulled(ctx context.Context, images []image) (map[string]bool, error) {
	if ok, err := s.docker.containerdStore(ctx); err != nil || !ok {
		return nil, err
	}
	out := map[string]bool{}
	for _, im := range images {
		if im.Containers != 0 || len(im.RepoDigests) == 0 {
			continue
		}
		ms, err := s.docker.manifests(ctx, im.ID)
		if errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[im.ID] = slices.ContainsFunc(ms, func(m manifest) bool { return !m.Available })
	}
	return out, nil
}

// unusedImages are images no container uses that a registry can send
// again. One without a digest was built or loaded here.
func unusedImages(w *world) []Find {
	var out []Find
	for _, im := range w.usage.Images {
		if im.Containers != 0 || len(im.RepoDigests) == 0 || (w.pulled != nil && !w.pulled[im.ID]) {
			continue
		}
		name := ""
		if tags := im.tags(); len(tags) > 0 {
			name = tags[0]
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
