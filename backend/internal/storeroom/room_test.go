package storeroom

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func (b *box) run(op string, refs ...Ref) []Step {
	b.t.Helper()
	steps, err := b.svc.Run(b.t.Context(), op, refs)
	if err != nil {
		b.t.Fatal(err)
	}
	return steps
}

func (b *box) entry(kind, name string) Ref {
	b.t.Helper()
	for _, e := range b.scan().Room {
		if e.Kind == kind && e.Name == name {
			return Ref{ID: e.ID}
		}
	}
	b.t.Fatalf("no %s %s in the storeroom", kind, name)
	return Ref{}
}

func done(t *testing.T, steps []Step) {
	t.Helper()
	for _, s := range steps {
		if !s.Done || s.Problem != "" {
			t.Fatalf("step %+v did not happen", s)
		}
	}
}

func TestThePlanSaysWhatHappensAndTouchesNothing(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	dir := b.appdata("verwaist", 200*day)
	b.engine.df.Images = append(b.engine.df.Images, struct {
		ID         string `json:"Id"`
		RepoTags   []string
		Size       int64
		Containers int64
	}{ID: "sha256:aa", RepoTags: []string{"alt:latest"}, Size: 100})
	s := b.scan()
	steps, err := b.svc.Plan(t.Context(), "store", []Ref{find(t, s, "template", "alt"), find(t, s, "appdata", "verwaist"), find(t, s, "image", "alt:latest")})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].From != tpl || !strings.HasPrefix(steps[0].To, b.cfg.Room) || steps[0].ForGood {
		t.Errorf("template step = %+v", steps[0])
	}
	if steps[1].From != dir || !strings.HasPrefix(steps[1].To, filepath.Join(b.cfg.Appdata, ".storeroom")) {
		t.Errorf("folder step = %+v", steps[1])
	}
	if !steps[2].ForGood || steps[2].To != "" {
		t.Errorf("image step = %+v", steps[2])
	}
	if !exists(tpl) || !exists(dir) || len(b.engine.log()) != 0 {
		t.Error("the plan changed something")
	}
}

func TestATemplateGoesIntoTheStoreroomAndComesBack(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	done(t, b.run("store", find(t, b.scan(), "template", "alt")))
	if exists(tpl) || len(found(b.scan(), "template")) != 0 {
		t.Fatal("the template is still in place")
	}
	done(t, b.run("restore", b.entry("template", "alt")))
	if !exists(tpl) || len(b.scan().Room) != 0 {
		t.Error("the template did not come back")
	}
}

func TestAFolderGoesIntoTheStoreroomOnItsOwnShareAndComesBack(t *testing.T) {
	b := newBox(t)
	dir := b.appdata("verwaist", 200*day)
	steps := b.run("store", find(t, b.scan(), "appdata", "verwaist"))
	done(t, steps)
	if exists(dir) || !exists(filepath.Join(steps[0].To, "settings.json")) {
		t.Fatalf("after storing: %s %v, %s %v", dir, exists(dir), steps[0].To, exists(steps[0].To))
	}
	if got := found(b.scan(), "appdata"); len(got) != 0 {
		t.Errorf("the storeroom itself counts as a folder: %v", got)
	}
	done(t, b.run("restore", b.entry("appdata", "verwaist")))
	if !exists(filepath.Join(dir, "settings.json")) {
		t.Error("the folder did not come back")
	}
}

func TestEmptyingTheStoreroomRemovesForGood(t *testing.T) {
	b := newBox(t)
	b.appdata("verwaist", 200*day)
	steps := b.run("store", find(t, b.scan(), "appdata", "verwaist"))
	done(t, b.run("discard", b.entry("appdata", "verwaist")))
	if exists(steps[0].To) || len(b.scan().Room) != 0 {
		t.Error("the folder is still in the storeroom")
	}
}

func TestNothingComesBackOverSomethingNew(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	b.run("store", find(t, b.scan(), "template", "alt"))
	b.write(tpl, "new", 0)
	steps := b.run("restore", b.entry("template", "alt"))
	if steps[0].Done || steps[0].Problem != "taken" {
		t.Errorf("restore over a new file = %+v", steps[0])
	}
	if raw, _ := os.ReadFile(tpl); string(raw) != "new" {
		t.Error("the new file was replaced")
	}
}

func TestAFindThatIsInUseAgainIsLeftAlone(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	ref := find(t, b.scan(), "template", "alt")
	b.engine.containers = []container{ctr("a1", "alt", "running", day, nil)}
	steps := b.run("store", ref)
	if steps[0].Done || steps[0].Problem != "inUse" || !exists(tpl) {
		t.Errorf("store of a template in use = %+v", steps[0])
	}
}

func TestNothingMovesWithoutWriteAccess(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	ref := find(t, b.scan(), "template", "alt")
	b.svc.cfg.Write = false
	if _, err := b.svc.Run(t.Context(), "store", []Ref{ref}); !errors.Is(err, ErrWriteOff) {
		t.Errorf("err = %v", err)
	}
	if !exists(tpl) {
		t.Error("the template moved")
	}
}

func TestALeftoverContainerIsRenamedAwayAndBack(t *testing.T) {
	b := newBox(t)
	b.engine.containers = []container{ctr("c1", "uebrig", "exited", 60*day, nil)}
	done(t, b.run("store", find(t, b.scan(), "container", "uebrig")))
	held := b.engine.containers[0].name()
	if !strings.HasPrefix(held, "storeroom-") || len(found(b.scan(), "container")) != 0 {
		t.Fatalf("held as %q", held)
	}
	done(t, b.run("restore", b.entry("container", "uebrig")))
	if b.engine.containers[0].name() != "uebrig" {
		t.Errorf("came back as %q", b.engine.containers[0].name())
	}
}

func TestADiscardedContainerIsRemovedWithoutItsVolumes(t *testing.T) {
	b := newBox(t)
	b.engine.containers = []container{ctr("c1", "uebrig", "exited", 60*day, nil)}
	b.run("store", find(t, b.scan(), "container", "uebrig"))
	done(t, b.run("discard", b.entry("container", "uebrig")))
	if log := b.engine.log(); log[len(log)-1] != "rm container c1" {
		t.Errorf("engine log %v", log)
	}
}

func TestANetworkIsKeptAsItsSettingsAndMadeAgain(t *testing.T) {
	b := newBox(t)
	b.engine.networks = []network{{ID: "n5", Name: "alt", Driver: "bridge", Scope: "local", IPAM: json.RawMessage(`{"Config":[{"Subnet":"172.30.0.0/16"}]}`)}}
	done(t, b.run("store", find(t, b.scan(), "network", "alt")))
	if len(b.engine.networks) != 0 {
		t.Fatal("the network is still there")
	}
	done(t, b.run("restore", b.entry("network", "alt")))
	log := b.engine.log()
	if !slices.Contains(log, "rm network n5") || log[len(log)-1] != `create network alt bridge {"Config":[{"Subnet":"172.30.0.0/16"}]}` {
		t.Errorf("engine log %v", log)
	}
}

func TestAVolumeWaitsInTheStoreroomUntilItIsEmptied(t *testing.T) {
	b := newBox(t)
	anon := "3f1b5a1c8d2e4f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8"
	b.engine.volumes = []volume{{Name: anon, Driver: "local"}}
	done(t, b.run("store", find(t, b.scan(), "volume", anon)))
	if len(b.engine.log()) != 0 || len(found(b.scan(), "volume")) != 0 {
		t.Fatalf("storing a volume: log %v", b.engine.log())
	}
	done(t, b.run("discard", b.entry("volume", anon)))
	if log := b.engine.log(); !slices.Equal(log, []string{"rm volume " + anon}) {
		t.Errorf("engine log %v", log)
	}
}

func TestImagesAndTheBuildCacheSkipTheStoreroom(t *testing.T) {
	b := newBox(t)
	b.engine.df.Images = append(b.engine.df.Images, struct {
		ID         string `json:"Id"`
		RepoTags   []string
		Size       int64
		Containers int64
	}{ID: "sha256:aa", RepoTags: []string{"alt:latest"}, Size: 100})
	b.engine.df.BuildCache = append(b.engine.df.BuildCache, struct {
		ID    string
		Size  int64
		InUse bool
	}{ID: "x", Size: 30})
	s := b.scan()
	done(t, b.run("store", find(t, s, "image", "alt:latest"), find(t, s, "cache", "")))
	if log := b.engine.log(); !slices.Equal(log, []string{"rm image sha256:aa", "prune build cache"}) {
		t.Errorf("engine log %v", log)
	}
	if len(b.scan().Room) != 0 {
		t.Error("images landed in the storeroom")
	}
}

func TestTheStoreroomOnlyEmptiesWhatLiesInIt(t *testing.T) {
	b := newBox(t)
	b.appdata("verwaist", 200*day)
	b.run("store", find(t, b.scan(), "appdata", "verwaist"))
	outside := b.appdata("wichtig", 0)
	idx := filepath.Join(b.cfg.Room, "index.json")
	raw, err := os.ReadFile(idx)
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Entries []map[string]any }
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	file.Entries[0]["held"] = outside
	raw, _ = json.Marshal(file)
	b.write(idx, string(raw), 0)
	b.svc = New(b.cfg)
	b.svc.docker = nil
	steps, err := b.svc.Run(t.Context(), "discard", []Ref{b.entry("appdata", "verwaist")})
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Done || !exists(outside) {
		t.Errorf("discard reached outside the storeroom: %+v", steps[0])
	}
}
