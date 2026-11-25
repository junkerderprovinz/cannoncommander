package storeroom

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/dockerengine"
)

// ErrWriteOff is returned while write access is switched off.
var ErrWriteOff = errors.New("storeroom: write access is switched off")

// ErrBadOp is returned for an operation other than store, restore or discard.
var ErrBadOp = errors.New("storeroom: unknown operation")

var errNoDocker = errors.New("storeroom: docker does not answer")

// Entry is one thing in the storeroom. From is where it came from: a path,
// a container's name, a network's name or a volume's name. Held is where it
// waits: a path in the storeroom, or the name a container waits under.
type Entry struct {
	ID   string    `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name"`
	From string    `json:"from"`
	Held string    `json:"held"`
	At   time.Time `json:"at"`
	Size int64     `json:"size"`
	// Ref is the engine's ID of a container in the storeroom.
	Ref string `json:"ref,omitempty"`
	// Network is what makes a network again: its settings as the engine
	// takes them to create one.
	Network json.RawMessage `json:"network,omitempty"`
}

// Ref picks a find by kind and ID, or an entry of the storeroom by its ID.
type Ref struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id"`
}

// Step is what an operation does with one ref. Problem names why it is
// left out: gone, inUse, taken or failed.
type Step struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	From    string `json:"from"`
	To      string `json:"to,omitempty"`
	ForGood bool   `json:"forGood"`
	Done    bool   `json:"done"`
	Problem string `json:"problem,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (s *Service) index() string { return filepath.Join(s.cfg.Room, "index.json") }

// folders is where appdata folders wait, on their own share.
func (s *Service) folders() string { return filepath.Join(s.cfg.Appdata, ".storeroom") }

func (s *Service) load() ([]Entry, error) {
	raw, err := os.ReadFile(s.index())
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file struct{ Entries []Entry }
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	if file.Entries == nil {
		file.Entries = []Entry{}
	}
	return file.Entries, nil
}

func (s *Service) save(entries []Entry) error {
	raw, err := json.MarshalIndent(map[string]any{"entries": entries}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.cfg.Room, 0o755); err != nil {
		return err
	}
	tmp := s.index() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.index())
}

// Plan says what op would do with refs, without doing any of it.
func (s *Service) Plan(ctx context.Context, op string, refs []Ref) ([]Step, error) {
	return s.apply(ctx, op, refs, false)
}

// Run does op with refs: store moves finds into the storeroom or, for images
// and the build cache, removes them; restore brings entries back; discard
// removes entries for good. Each find is checked again first, and one that
// is in use by now is left alone.
func (s *Service) Run(ctx context.Context, op string, refs []Ref) ([]Step, error) {
	if !s.cfg.Write {
		return nil, ErrWriteOff
	}
	return s.apply(ctx, op, refs, true)
}

func (s *Service) apply(ctx context.Context, op string, refs []Ref, act bool) ([]Step, error) {
	if op != "store" && op != "restore" && op != "discard" {
		return nil, ErrBadOp
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, err := s.load()
	if err != nil {
		return nil, err
	}
	steps := []Step{}
	if op == "store" {
		w, err := s.look(ctx, room)
		if err != nil {
			return nil, err
		}
		finds := s.finds(w)
		for _, r := range refs {
			st, e := s.store(ctx, w, finds, r, act)
			steps = append(steps, st)
			if e != nil {
				room = append(room, *e)
			}
		}
	} else {
		for _, r := range refs {
			i := indexOf(room, r.ID)
			if i < 0 {
				steps = append(steps, Step{Problem: "gone"})
				continue
			}
			st, keep := s.release(ctx, op, room[i], act)
			steps = append(steps, st)
			if !keep {
				room = append(room[:i], room[i+1:]...)
			}
		}
	}
	if act {
		return steps, s.save(room)
	}
	return steps, nil
}

func indexOf(room []Entry, id string) int {
	for i, e := range room {
		if e.ID == id {
			return i
		}
	}
	return -1
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func failed(st Step, err error) Step {
	st.Problem, st.Error = "failed", err.Error()
	return st
}

// store plans or does the store of one find and returns the entry it makes.
func (s *Service) store(ctx context.Context, w *world, finds []Find, r Ref, act bool) (Step, *Entry) {
	var f *Find
	for i := range finds {
		if finds[i].Kind == r.Kind && finds[i].ID == r.ID {
			f = &finds[i]
		}
	}
	if f == nil {
		return Step{Kind: r.Kind, From: r.ID, Problem: "inUse"}, nil
	}
	id := newID()
	e := &Entry{ID: id, Kind: f.Kind, Name: f.Name, From: f.ID, At: s.now().UTC(), Size: f.Size}
	st := Step{Kind: f.Kind, Name: f.Name, From: f.ID, ForGood: f.ForGood}
	switch f.Kind {
	case "template":
		e.Held = filepath.Join(s.cfg.Room, "templates", id+"-"+filepath.Base(f.ID))
	case "stack":
		e.Held = filepath.Join(s.cfg.Room, "stacks", id+"-"+filepath.Base(f.ID))
	case "appdata":
		e.Held = filepath.Join(s.folders(), id, f.Name)
	case "container":
		e.From, e.Ref, e.Held = f.Name, f.ID, "storeroom-"+id+"-"+f.Name
		st.From = f.Name
	case "network":
		e.From, e.Held = f.Name, f.Name
		st.From = f.Name
	case "volume":
		e.Held = f.ID
	case "image", "cache":
		e = nil
	}
	if e != nil && e.Held != f.ID {
		st.To = e.Held
	}
	if !act {
		return st, nil
	}
	var err error
	switch f.Kind {
	case "template", "stack":
		err = move(f.ID, e.Held)
	case "appdata":
		if err = os.MkdirAll(filepath.Dir(e.Held), 0o755); err == nil {
			err = os.Rename(f.ID, e.Held)
		}
	case "container":
		err = s.docker.rename(ctx, f.ID, e.Held)
	case "network":
		var n network
		if n, err = s.docker.network(ctx, f.ID); err == nil {
			e.Network, err = json.Marshal(map[string]any{
				"Name": n.Name, "Driver": n.Driver, "Internal": n.Internal, "Attachable": n.Attachable,
				"EnableIPv6": n.EnableIPv6, "IPAM": n.IPAM, "Options": n.Options, "Labels": n.Labels,
			})
		}
		if err == nil {
			err = s.docker.removeNetwork(ctx, f.ID)
		}
	case "image":
		err = s.removeImage(ctx, w, f.ID)
	case "cache":
		err = s.docker.pruneBuildCache(ctx)
	}
	if err != nil {
		return failed(st, err), nil
	}
	st.Done = true
	return st, e
}

// release plans or does the restore or discard of one entry. keep is true
// while the entry stays in the storeroom.
func (s *Service) release(ctx context.Context, op string, e Entry, act bool) (st Step, keep bool) {
	st = Step{Kind: e.Kind, Name: e.Name, From: e.Held, ForGood: op == "discard"}
	if op == "restore" {
		st.To = e.From
	}
	switch e.Kind {
	case "template", "stack", "appdata":
		if !s.holds(e) {
			return failed(st, fmt.Errorf("%s lies outside the storeroom", e.Held)), true
		}
		if op == "restore" && exists(e.From) {
			st.Problem = "taken"
			return st, true
		}
	case "container", "network", "volume":
		if s.docker == nil {
			return failed(st, errNoDocker), true
		}
	}
	if !act {
		return st, true
	}
	var err error
	switch {
	case op == "restore" && (e.Kind == "template" || e.Kind == "stack"):
		err = move(e.Held, e.From)
	case op == "restore" && e.Kind == "appdata":
		if err = os.Rename(e.Held, e.From); err == nil {
			_ = os.Remove(filepath.Dir(e.Held))
		}
	case op == "restore" && e.Kind == "container":
		err = s.docker.rename(ctx, e.Ref, e.From)
	case op == "restore" && e.Kind == "network":
		err = s.docker.createNetwork(ctx, e.Network)
	case op == "restore" && e.Kind == "volume":
	case e.Kind == "template":
		err = os.Remove(e.Held)
	case e.Kind == "stack":
		err = os.RemoveAll(e.Held)
	case e.Kind == "appdata":
		err = os.RemoveAll(filepath.Dir(e.Held))
	case e.Kind == "container":
		err = s.docker.removeContainer(ctx, e.Ref)
	case e.Kind == "network":
	case e.Kind == "volume":
		err = s.docker.removeVolume(ctx, e.Held)
	}
	if errors.Is(err, dockerengine.ErrNotFound) && op == "discard" {
		err = nil
	}
	if err != nil {
		return failed(st, err), true
	}
	st.Done = true
	return st, false
}

// removeImage takes each name off first when there are several, since the
// engine refuses to remove an image by its ID that more than one repository
// names.
func (s *Service) removeImage(ctx context.Context, w *world, id string) error {
	for _, im := range w.usage.Images {
		if im.ID != id || len(im.tags()) < 2 {
			continue
		}
		for _, tag := range im.tags() {
			if err := s.docker.removeImage(ctx, tag); err != nil && !errors.Is(err, dockerengine.ErrNotFound) {
				return err
			}
		}
	}
	if err := s.docker.removeImage(ctx, id); err != nil && !errors.Is(err, dockerengine.ErrNotFound) {
		return err
	}
	return nil
}

// holds reports whether a template, stack or folder entry waits inside the
// storeroom. The index is a file anyone with the config share can edit, and
// discarding removes whatever it points at.
func (s *Service) holds(e Entry) bool {
	base, depth := filepath.Join(s.cfg.Room, "templates"), 1
	switch e.Kind {
	case "stack":
		base = filepath.Join(s.cfg.Room, "stacks")
	case "appdata":
		base, depth = s.folders(), 2
	}
	rel, err := filepath.Rel(base, e.Held)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == depth && !slices.ContainsFunc(parts, func(p string) bool { return p == "" || p == "." || p == ".." })
}

// rename is os.Rename, which tests swap for one that fails as between mounts.
var rename = os.Rename

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// move renames a file or folder, or copies it where the storeroom lies on
// another file system than the flash. It never writes over what is at to.
func move(from, to string) error {
	if exists(to) {
		return fmt.Errorf("%s: %w", to, fs.ErrExist)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if rename(from, to) == nil {
		return nil
	}
	if err := copyAll(from, to); err != nil {
		os.RemoveAll(to)
		return err
	}
	if err := os.RemoveAll(from); err != nil {
		// While nothing of from is gone the copy is only in the way, and
		// once something is, the copy is all that is left of it.
		if whole(from, to) {
			os.RemoveAll(to)
		}
		return err
	}
	return nil
}

// copyAll copies a file or a whole folder to a path that does not exist yet,
// with the times it had, since a stack counts as unused by how long it went
// unchanged.
func copyAll(from, to string) error {
	var made []string
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		switch {
		case d.IsDir():
			err = os.Mkdir(dst, 0o755)
		case d.Type().IsRegular():
			err = copyFile(p, dst)
		default:
			return fmt.Errorf("%s is neither a file nor a folder", p)
		}
		if err != nil {
			return err
		}
		made = append(made, rel)
		return nil
	})
	if err != nil {
		return err
	}
	// Each file written into a folder changed its time, so the times go on last.
	for _, rel := range made {
		info, err := os.Stat(filepath.Join(from, rel))
		if err != nil {
			return err
		}
		if err := os.Chtimes(filepath.Join(to, rel), time.Time{}, info.ModTime()); err != nil {
			return err
		}
	}
	return nil
}

// whole reports whether everything copied to to is still at from.
func whole(from, to string) bool {
	return filepath.WalkDir(to, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(to, p)
		if err != nil {
			return err
		}
		_, err = os.Lstat(filepath.Join(from, rel))
		return err
	}) == nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
