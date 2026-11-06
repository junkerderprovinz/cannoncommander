package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
)

type devicesFeedFunc func(func([]string))

func (f devicesFeedFunc) OnDevices(hear func([]string)) { f(hear) }

func TestADiskThatComesOrGoesReachesTheTreeAndTheOpenFolders(t *testing.T) {
	mnt := t.TempDir()
	for _, d := range []string{"user", "disk1", "disk2"} {
		if err := os.MkdirAll(filepath.Join(mnt, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plain, err := ccfs.New(nil, ccfs.Source{Name: "Shares", Mount: "/mnt/user", Root: filepath.Join(mnt, "user")})
	if err != nil {
		t.Fatal(err)
	}
	fs := plain.WithDevices(mnt, "disk1")
	h := hub.New(hub.Options{})
	c, _, _ := h.Connect("tab", "jdp", 0)
	h.Subscribe("tab", "jdp", hub.FolderTopic("/mnt"), hub.FolderTopic("/mnt/disk1"))

	var hear func([]string)
	followDevices(devicesFeedFunc(func(f func([]string)) { hear = f }), fs, h, []string{"disk1"})
	hear([]string{"disk2"})

	if got := fs.VirtualChildren("/mnt"); !slices.Equal(got, []string{"user", "disk2"}) {
		t.Errorf("/mnt lists %v", got)
	}
	changes := map[string]watch.Change{}
	timeout := time.After(5 * time.Second)
	for len(changes) < 2 {
		select {
		case ev := <-c.Events:
			if ev.Type != watch.EventChanged {
				continue
			}
			var ch watch.Change
			if err := json.Unmarshal(ev.Data, &ch); err != nil {
				t.Fatal(err)
			}
			changes[ch.Path] = ch
		case <-timeout:
			t.Fatalf("only %v arrived", changes)
		}
	}
	if !changes["/mnt"].Reload || !changes["/mnt/disk1"].Gone {
		t.Errorf("changes %+v", changes)
	}
}
