//go:build desktop

package places

import (
	"path"
	"path/filepath"
	"slices"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// Follow lets fs reach what the desktop app sees of the computer: the shares
// of other computers by their path, and the drives, which it keeps in step
// as they come and go.
func Follow(fs *ccfs.Resolver, h *hub.Hub) *ccfs.Resolver {
	if n := network(); n != nil {
		fs = fs.WithNetwork(n)
	}
	vols := desktop.CurrentVolumes()
	if vols == nil {
		return fs
	}
	f := &follower{fs: fs, hub: h, vols: vols, known: vols.Volumes()}
	fs.SetVolumes(sources(f.known))
	vols.OnVolumes(f.changed)
	return fs
}

type follower struct {
	fs   *ccfs.Resolver
	hub  *hub.Hub
	vols desktop.Volumes

	mu    sync.Mutex
	known []desktop.Volume
}

// volumesEvent is what the window hears as desktop.volumes. It reads the
// places again, greets a drive that came and leaves one that went.
type volumesEvent struct {
	Added   []volumeJSON `json:"added"`
	Removed []volumeJSON `json:"removed"`
}

func (f *follower) changed() {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.vols.Volumes()
	if slices.Equal(f.known, now) {
		return
	}
	added, removed := compare(f.known, now)
	f.known = now
	f.fs.SetVolumes(sources(now))
	ev := volumesEvent{Added: []volumeJSON{}, Removed: []volumeJSON{}}
	folders := []string{"/"}
	for _, v := range added {
		j := volumeFor(v)
		ev.Added = append(ev.Added, j)
		if j.Path != "" {
			folders = append(folders, path.Dir(j.Path))
		}
	}
	var gone []string
	for _, v := range removed {
		j := volumeFor(v)
		ev.Removed = append(ev.Removed, j)
		if j.Path != "" {
			gone = append(gone, j.Path)
			folders = append(folders, path.Dir(j.Path))
		}
	}
	logger.Infof("drives: %d came, %d went", len(added), len(removed))
	// The window moves away from a drive that went before the folders it
	// showed are found missing.
	desktop.Emit("volumes", ev)
	for _, p := range gone {
		f.publish(watch.Change{Path: p, Paths: []string{p}, Gone: true})
	}
	slices.Sort(folders)
	for _, p := range slices.Compact(folders) {
		f.publish(watch.Change{Path: p, Paths: []string{p}, Reload: true})
	}
}

func (f *follower) publish(c watch.Change) {
	if _, err := f.hub.PublishJSON(watch.EventChanged, hub.FolderTopic(c.Path), c); err != nil {
		logger.Errorf("publish %s: %v", c.Path, err)
	}
}

// compare returns the volumes that came or were mounted, and those that
// went or were unmounted, between before and now.
func compare(before, now []desktop.Volume) (added, removed []desktop.Volume) {
	find := func(list []desktop.Volume, v desktop.Volume) (desktop.Volume, bool) {
		at := slices.IndexFunc(list, func(o desktop.Volume) bool { return o.ID == v.ID })
		if at < 0 {
			return desktop.Volume{}, false
		}
		return list[at], true
	}
	for _, v := range now {
		if old, ok := find(before, v); !ok || old.Root == "" && v.Root != "" {
			added = append(added, v)
		}
	}
	for _, v := range before {
		if cur, ok := find(now, v); !ok || v.Root != "" && cur.Root == "" {
			removed = append(removed, v)
		}
	}
	return added, removed
}

// sources are the mounted volumes as the resolver takes them, named by
// their letter on Windows and by their folder elsewhere.
func sources(vols []desktop.Volume) []ccfs.Volume {
	var out []ccfs.Volume
	for _, v := range vols {
		if v.Root == "" {
			continue
		}
		name := v.ID
		if filepath.VolumeName(v.Root) == "" {
			name = filepath.Base(v.Root)
		}
		out = append(out, ccfs.Volume{Name: name, Root: v.Root, Drive: ccfs.Drive{Label: v.Label, Kind: v.Kind, Remote: v.Remote, Eject: v.Eject}})
	}
	return out
}
