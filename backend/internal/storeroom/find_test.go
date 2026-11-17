package storeroom

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

func TestATemplateIsUnusedWhenNoContainerHasItsName(t *testing.T) {
	b := newBox(t)
	b.template("plex", "")
	b.template("alt", "")
	b.template("gestoppt", "")
	b.write(filepath.Join(b.cfg.Templates, "kaputt.xml"), "<Container><Name>", 90*day)
	b.write(filepath.Join(b.cfg.Templates, "my-ohnename.xml"), "<Container></Container>", 90*day)
	b.engine.containers = []container{
		ctr("a1", "plex", "running", 90*day, nil),
		ctr("a2", "gestoppt", "exited", 90*day, nil),
	}
	if got := found(b.scan(), "template"); !slices.Equal(got, []string{"alt"}) {
		t.Errorf("unused templates = %v", got)
	}
}

func TestWithoutDockerNoTemplateOrFolderCountsAsUnused(t *testing.T) {
	b := newBox(t)
	b.template("alt", "")
	b.appdata("verwaist", 200*day)
	b.svc.docker = nil
	s := b.scan()
	if s.Docker || len(s.Finds) != 0 {
		t.Errorf("without docker: docker %v, finds %+v", s.Docker, s.Finds)
	}
}

func TestAnAppdataFolderStaysWhileAnythingNamesIt(t *testing.T) {
	b := newBox(t)
	b.appdata("plex", 200*day)
	b.appdata("cache-mount", 200*day)
	b.appdata("vorlage", 200*day)
	b.appdata("stapel", 200*day)
	b.appdata("plugin", 200*day)
	b.appdata("frisch", 3*day)
	b.appdata("verwaist", 200*day)
	b.appdata("auch-verwaist", 400*day)
	b.appdata(".versteckt", 200*day)
	old := b.appdata("innen-frisch", 200*day)
	b.write(filepath.Join(old, "db.sqlite"), "x", 2*day)
	b.age(old, 200*day)
	b.template("vorlage-ohne-container", `  <Config Name="Config" Target="/config" Default="/mnt/user/appdata/vorlage" Mode="rw">/mnt/user/appdata/vorlage/</Config>`+"\n")
	b.write(filepath.Join(b.cfg.Plugins, "compose.manager/projects/stapel/docker-compose.yml"), "services:\n  app:\n    volumes:\n      - /mnt/cache/appdata/stapel:/data\n", 90*day)
	b.write(filepath.Join(b.cfg.Plugins, "something/something.cfg"), `KEEP="/mnt/user/appdata/plugin"`+"\n", 90*day)
	b.engine.containers = []container{
		ctr("a1", "plex", "running", 90*day, nil, "/mnt/user/appdata/plex/config", "/mnt/user/filme"),
		ctr("a2", "other", "exited", 90*day, nil, "/mnt/cache/appdata/cache-mount"),
		ctr("cc", "CannonCommander", "running", 90*day, nil, "/mnt", "/mnt/user/appdata"),
	}
	b.template("plex", "")
	b.template("other", "")
	b.template("CannonCommander", "")
	got := found(b.scan(), "appdata")
	slices.Sort(got)
	if !slices.Equal(got, []string{"auch-verwaist", "verwaist"}) {
		t.Errorf("orphaned folders = %v", got)
	}
}

func TestAnAppdataFolderNamedLikeAnotherIsStillItsOwn(t *testing.T) {
	b := newBox(t)
	b.appdata("app", 200*day)
	b.engine.containers = []container{ctr("a1", "app2", "running", 90*day, nil, "/mnt/user/appdata/app2")}
	b.template("app2", "")
	if got := found(b.scan(), "appdata"); !slices.Equal(got, []string{"app"}) {
		t.Errorf("orphaned folders = %v", got)
	}
}

func TestOnlyAStoppedContainerNothingManagesIsALeftover(t *testing.T) {
	b := newBox(t)
	b.template("mit-vorlage", "")
	b.engine.containers = []container{
		ctr("c1", "uebrig", "exited", 60*day, nil),
		ctr("c2", "angelegt", "created", 60*day, nil),
		ctr("c3", "unraid", "exited", 60*day, map[string]string{"net.unraid.docker.managed": "dockerman"}),
		ctr("c4", "stapel", "exited", 60*day, map[string]string{"com.docker.compose.project": "web"}),
		ctr("c5", "jung", "exited", 5*day, nil),
		ctr("c6", "laeuft", "running", 60*day, nil),
		ctr("c7", "mit-vorlage", "exited", 60*day, nil),
		ctr("c8", "pausiert", "paused", 60*day, nil),
	}
	if got := found(b.scan(), "container"); !slices.Equal(got, []string{"angelegt", "uebrig"}) {
		t.Errorf("leftover containers = %v", got)
	}
}

func TestANetworkNobodyUsesIsOnlyAPlainOneOfOurOwn(t *testing.T) {
	b := newBox(t)
	b.template("proxy", "  <Network>proxynet</Network>\n")
	used := ctr("c1", "gestoppt", "exited", 60*day, map[string]string{"net.unraid.docker.managed": "dockerman"})
	used.NetworkSettings.Networks = map[string]json.RawMessage{"backend": json.RawMessage(`{}`)}
	mode := ctr("c2", "modus", "exited", 60*day, map[string]string{"net.unraid.docker.managed": "dockerman"})
	mode.HostConfig.NetworkMode = "intern"
	b.engine.containers = []container{used, mode}
	b.engine.networks = []network{
		{ID: "n1", Name: "bridge", Driver: "bridge", Scope: "local"},
		{ID: "n2", Name: "host", Driver: "host", Scope: "local"},
		{ID: "n3", Name: "none", Driver: "null", Scope: "local"},
		{ID: "n4", Name: "br0", Driver: "macvlan", Scope: "local"},
		{ID: "n5", Name: "alt", Driver: "bridge", Scope: "local"},
		{ID: "n6", Name: "backend", Driver: "bridge", Scope: "local"},
		{ID: "n7", Name: "intern", Driver: "bridge", Scope: "local"},
		{ID: "n8", Name: "web_default", Driver: "bridge", Scope: "local", Labels: map[string]string{"com.docker.compose.network": "default"}},
		{ID: "n9", Name: "proxynet", Driver: "bridge", Scope: "local"},
		{ID: "n10", Name: "aktiv", Driver: "bridge", Scope: "local", Containers: map[string]json.RawMessage{"x": json.RawMessage(`{}`)}},
	}
	if got := found(b.scan(), "network"); !slices.Equal(got, []string{"alt"}) {
		t.Errorf("unused networks = %v", got)
	}
}

func TestOnlyAnAnonymousVolumeNobodyHoldsIsALeftover(t *testing.T) {
	b := newBox(t)
	anon := "3f1b5a1c8d2e4f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8"
	b.engine.volumes = []volume{
		{Name: anon, Driver: "local"},
		{Name: "datenbank", Driver: "local"},
		{Name: "a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1", Driver: "local", Labels: map[string]string{"com.docker.compose.project": "web"}},
	}
	b.engine.df.Volumes = []volume{{Name: anon, UsageData: &struct{ Size int64 }{Size: 4096}}}
	s := b.scan()
	if got := found(s, "volume"); !slices.Equal(got, []string{anon}) {
		t.Errorf("leftover volumes = %v", got)
	}
	for _, f := range s.Finds {
		if f.Kind == "volume" && f.Size != 4096 {
			t.Errorf("volume size %d", f.Size)
		}
	}
}

func TestImagesNoContainerUsesAndTheBuildCacheGoForGood(t *testing.T) {
	b := newBox(t)
	b.engine.df.Images = append(b.engine.df.Images,
		struct {
			ID         string `json:"Id"`
			RepoTags   []string
			Size       int64
			Containers int64
		}{ID: "sha256:aa", RepoTags: []string{"lscr.io/alt:latest"}, Size: 100},
		struct {
			ID         string `json:"Id"`
			RepoTags   []string
			Size       int64
			Containers int64
		}{ID: "sha256:bb", RepoTags: []string{"<none>:<none>"}, Size: 50},
		struct {
			ID         string `json:"Id"`
			RepoTags   []string
			Size       int64
			Containers int64
		}{ID: "sha256:cc", RepoTags: []string{"plex:latest"}, Size: 900, Containers: 1},
	)
	b.engine.df.BuildCache = append(b.engine.df.BuildCache,
		struct {
			ID    string
			Size  int64
			InUse bool
		}{ID: "x", Size: 30},
		struct {
			ID    string
			Size  int64
			InUse bool
		}{ID: "y", Size: 70, InUse: true},
	)
	s := b.scan()
	if got := found(s, "image"); !slices.Equal(got, []string{"lscr.io/alt:latest", ""}) {
		t.Errorf("unused images = %v", got)
	}
	for _, f := range s.Finds {
		if (f.Kind == "image" || f.Kind == "cache") != f.ForGood {
			t.Errorf("%s %s for good = %v", f.Kind, f.Name, f.ForGood)
		}
		if f.Kind == "cache" && f.Size != 30 {
			t.Errorf("build cache = %d", f.Size)
		}
	}
	if got := found(s, "cache"); len(got) != 1 {
		t.Errorf("build cache finds = %v", got)
	}
}
