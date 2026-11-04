package settings

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestExcludePatternsBecomeIndexRules(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want ConditionalRule
	}{
		{"*.tmp", ConditionalRule{FileEndsWith: ".tmp", FolderEndsWith: ".tmp"}},
		{"backup-*", ConditionalRule{FileStartsWith: "backup-", FolderStartsWith: "backup-"}},
		{"Thumbs.db", ConditionalRule{FileName: "Thumbs.db", FolderName: "Thumbs.db"}},
		{"/appdata/plex/cache/", ConditionalRule{FolderPath: "appdata/plex/cache", FilePath: "appdata/plex/cache"}},
	} {
		got, err := ExcludeRule(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("%q: %+v, %v", tc.in, got, err)
		}
	}
	for _, bad := range []string{"", "*", "appdata/*/cache", "a*b", "*a*"} {
		if _, err := ExcludeRule(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestStoredSourceKeepsWhatThePageCannotShow(t *testing.T) {
	base := &Source{Path: "/mnt/user", Name: "user", Config: SourceConfig{
		DefaultUserScope: "/", CreateUserDir: true,
		Rules: []ConditionalRule{
			{NeverWatchPath: "/downloads"},
			{FolderPath: "/", IgnoreSymlinks: true},
			{FileEndsWith: ".part", FolderEndsWith: ".part"},
		},
	}}
	stored := StoredSource{Name: "Shares", Path: "/mnt/user", ReadOnly: true, Indexed: false, SkipHidden: true, Exclude: []string{"*.tmp", "appdata/cache"}}
	src, err := SourceFromStored(stored, base)
	if err != nil {
		t.Fatal(err)
	}
	if src.Name != "Shares" || !src.Config.ReadOnly || !src.Config.CreateUserDir || src.Config.DefaultUserScope != "/" {
		t.Fatalf("source %+v", src)
	}
	if !slices.Contains(src.Config.Rules, ConditionalRule{NeverWatchPath: "/downloads"}) {
		t.Error("the never watch rule is gone")
	}
	if !slices.Contains(src.Config.Rules, ConditionalRule{FolderPath: "/", Viewable: true, IgnoreHidden: true, IgnoreSymlinks: true}) {
		t.Errorf("root rule missing in %+v", src.Config.Rules)
	}
	if slices.Contains(src.Config.Rules, ConditionalRule{FileEndsWith: ".part", FolderEndsWith: ".part"}) {
		t.Error("an exclude the page dropped is still there")
	}
	if base.Config.ReadOnly || len(base.Config.Rules) != 3 {
		t.Error("the configured source was changed")
	}

	// The running source has its rules normalized, the way setupSources
	// leaves them.
	modifyExcludeInclude(src)
	back := DescribeSource(src)
	if !SameSources([]StoredSource{back}, []StoredSource{stored}) {
		t.Fatalf("round trip: %+v", back)
	}
}

func TestIndexedSourceWithoutFlagsHasNoRootRule(t *testing.T) {
	src, err := SourceFromStored(StoredSource{Name: "Cache", Path: "/mnt/cache", Indexed: true, Exclude: []string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Config.Rules) != 0 {
		t.Fatalf("rules %+v", src.Config.Rules)
	}
	if d := DescribeSource(src); !d.Indexed || d.SkipHidden || d.ReadOnly {
		t.Fatalf("described %+v", d)
	}
}

func TestRootRuleKeepsTheSourceInTheIndex(t *testing.T) {
	src := &Source{Path: "/mnt/user", Config: SourceConfig{Rules: []ConditionalRule{{FolderPath: "/", IgnoreHidden: true}}}}
	setConditionals(src)
	r := src.Config.ResolvedRules
	if !r.IgnoreAllHidden || r.IndexingDisabled || len(r.FolderPaths) != 0 {
		t.Fatalf("resolved %+v", r)
	}
}

func TestSavedSourcesReplaceTheConfiguredOnes(t *testing.T) {
	dir := t.TempDir()
	share, extra := filepath.Join(dir, "user"), filepath.Join(dir, "extra")
	for _, d := range []string{share, extra} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	saved := Config.Server.Sources
	t.Cleanup(func() { Config.Server.Sources = saved; sourcesFile = "" })
	Config.Server.Sources = []*Source{{Path: share, Name: "user", Config: SourceConfig{CreateUserDir: true}}}
	cfg := filepath.Join(dir, "config.yaml")
	list := []StoredSource{
		{Name: "Shares", Path: share, Indexed: true},
		{Name: "Extra", Path: extra, ReadOnly: true, Exclude: []string{"*.bak"}},
	}
	if err := WriteStoredSources(filepath.Join(dir, SourcesFileName), list); err != nil {
		t.Fatal(err)
	}
	useStoredSources(cfg)
	if SourcesFile() != filepath.Join(dir, SourcesFileName) {
		t.Fatalf("sources file %s", SourcesFile())
	}
	got := Config.Server.Sources
	if len(got) != 2 || got[0].Name != "Shares" || !got[0].Config.CreateUserDir || got[1].Name != "Extra" || !got[1].Config.ReadOnly {
		t.Fatalf("sources %+v %+v", got[0], got[1])
	}
}
