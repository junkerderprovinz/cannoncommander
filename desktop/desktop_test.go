package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestTheServerListensOnLoopbackWithoutSignIn(t *testing.T) {
	dir := t.TempDir()
	body, err := serverConfig(dir, 47321, []source{{Name: "C:", Path: `C:\`}, {Name: "jdp", Path: "/home/jdp", Indexed: true}})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		HTTP struct {
			Port   int    `json:"port"`
			Listen string `json:"listen"`
		} `json:"http"`
		Server struct {
			Database struct{ Path string } `json:"database"`
			Sources  []struct {
				Path   string
				Name   string
				Config struct {
					Rules []map[string]any
				}
			}
		}
		Auth struct {
			Methods struct {
				NoAuth bool `json:"noauth"`
			}
		}
	}
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 47321 || cfg.HTTP.Listen != "127.0.0.1" || !cfg.Auth.Methods.NoAuth {
		t.Fatalf("http %+v, noauth %v", cfg.HTTP, cfg.Auth.Methods.NoAuth)
	}
	if cfg.Server.Database.Path != filepath.Join(dir, "cannoncommander.sqlite") {
		t.Errorf("database at %s", cfg.Server.Database.Path)
	}
	if len(cfg.Server.Sources) != 2 || cfg.Server.Sources[0].Path != `C:\` {
		t.Fatalf("sources %+v", cfg.Server.Sources)
	}
	drive, home := cfg.Server.Sources[0].Config.Rules[0], cfg.Server.Sources[1].Config.Rules[0]
	if drive["viewable"] != true || home["viewable"] != nil || home["ignoreHidden"] != true {
		t.Errorf("a drive should be browsed without an index and the home searched: %v, %v", drive, home)
	}
}

func TestTheFirstStartWritesTheSourcesOnce(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeServerConfig(dir, 1, []source{{Name: "jdp", Path: "/home/jdp", Indexed: true}}); err != nil {
		t.Fatal(err)
	}
	stored := filepath.Join(dir, "sources.json")
	first, err := os.ReadFile(stored)
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	if err := json.Unmarshal(first, &list); err != nil || len(list) != 1 || list[0]["path"] != "/home/jdp" || list[0]["indexed"] != true {
		t.Fatalf("sources.json: %s", first)
	}
	// What the settings page saved stays, whatever the drives are now.
	if err := os.WriteFile(stored, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeServerConfig(dir, 2, []source{{Name: "D:", Path: `D:\`}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(stored); string(got) != "[]\n" {
		t.Fatalf("the second start rewrote sources.json: %s", got)
	}
}

func TestTheHomeComesFirstAndNoMountHoldsIt(t *testing.T) {
	exists := map[string]bool{"/media": true, "/mnt": true}
	got := homeAndMounts("/home/jdp", []string{"/media", "/run/media", "/mnt"}, func(p string) bool { return exists[p] })
	want := []source{{Name: "jdp", Path: "/home/jdp", Indexed: true}, {Name: "media", Path: "/media"}, {Name: "mnt", Path: "/mnt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	got = homeAndMounts("/mnt/users/jdp", []string{"/mnt"}, func(string) bool { return true })
	if len(got) != 1 {
		t.Fatalf("a mount folder that holds the home became a source: %+v", got)
	}
}

func TestTheSwitchIsOnUntilTurnedOff(t *testing.T) {
	dir := t.TempDir()
	s := openState(dir)
	if !s.get().autoUpdate() {
		t.Fatal("a fresh start updates")
	}
	off := false
	if err := s.update(func(st *State) { st.AutoUpdate = &off; st.Port = 47321 }); err != nil {
		t.Fatal(err)
	}
	again := openState(dir).get()
	if again.autoUpdate() || again.Port != 47321 {
		t.Fatalf("read back %+v", again)
	}
}

func TestTheInstallersSwitchKeepsItsOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if !readAutoUpdate(path) {
		t.Fatal("a missing file reads as on")
	}
	if err := os.WriteFile(path, []byte(`{"autoUpdate":true,"language":"de"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAutoUpdate(path, false); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	body, _ := os.ReadFile(path)
	if err := json.Unmarshal(body, &got); err != nil || got["autoUpdate"] != false || got["language"] != "de" {
		t.Fatalf("got %s", body)
	}
	if readAutoUpdate(path) {
		t.Fatal("read back on")
	}
}

func TestThePortOfTheLastStartIsKeptWhileItIsFree(t *testing.T) {
	free, err := pickPort(0)
	if err != nil || free == 0 {
		t.Fatalf("got %d, %v", free, err)
	}
	if got, _ := pickPort(free); got != free {
		t.Fatalf("got %d, want %d", got, free)
	}
	busy, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(free))
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if got, _ := pickPort(free); got == free || got == 0 {
		t.Fatalf("took the busy port: %d", got)
	}
}

func TestAWindowOffEveryScreenOpensInTheMiddle(t *testing.T) {
	screens := []*application.Screen{{WorkArea: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}}}
	if !onScreen(screens, Bounds{X: 100, Y: 80, Width: 1280, Height: 820}) {
		t.Error("a window on the screen was moved")
	}
	if onScreen(screens, Bounds{X: 2400, Y: 80, Width: 1280, Height: 820}) {
		t.Error("a window on a screen since unplugged kept its place")
	}
}

func TestACopyOnItsOwnDataFolderHasALockOfItsOwn(t *testing.T) {
	t.Setenv("CC_DATA", "")
	if instanceID("x") != appID {
		t.Fatal("the usual copy changed its lock")
	}
	t.Setenv("CC_DATA", "a")
	if a, b := instanceID("a"), instanceID("b"); a == appID || a == b {
		t.Fatalf("got %s and %s", a, b)
	}
}

type fakeWindow []string

func (f *fakeWindow) UnMinimise() { *f = append(*f, "unminimise") }
func (f *fakeWindow) Show() application.Window {
	*f = append(*f, "show")
	return nil
}
func (f *fakeWindow) Focus() { *f = append(*f, "focus") }

func TestAWindowComesUpAsLargeAsItWas(t *testing.T) {
	var w fakeWindow
	raise(&w)
	if want := []string{"unminimise", "show", "focus"}; !reflect.DeepEqual([]string(w), want) {
		t.Fatalf("got %v", w)
	}
}

// D-Bus refuses a name element that starts with a digit, and the Linux lock is
// a bus name, so such a copy would quit before its window opened.
func TestAPortableLockIsAValidBusName(t *testing.T) {
	t.Setenv("CC_DATA", "a")
	for i := range 64 {
		id := instanceID(fmt.Sprintf("/home/tester/data%d", i))
		for _, element := range strings.Split(id, ".") {
			if element == "" || element[0] >= '0' && element[0] <= '9' {
				t.Fatalf("%s is no bus name", id)
			}
		}
	}
}
