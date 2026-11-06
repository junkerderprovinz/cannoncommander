package web

import (
	"slices"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
)

// devicesFeed tells when Unraid mounts or unmounts a pool or disk.
type devicesFeed interface {
	OnDevices(f func(devices []string))
}

// followDevices keeps the pools and disks of fs in step with feed, starting
// from devices. Clients that show /mnt read it again, and those that show a
// device that went away hear that it is gone.
func followDevices(feed devicesFeed, fs *ccfs.Resolver, h *hub.Hub, devices []string) {
	feed.OnDevices(func(now []string) {
		fs.SetDevices(now...)
		logger.Infof("unraid: pools and disks are now %v", now)
		publishChange(h, watch.Change{Path: "/mnt", Paths: []string{"/mnt"}, Reload: true})
		for _, d := range devices {
			if !slices.Contains(now, d) {
				p := "/mnt/" + d
				publishChange(h, watch.Change{Path: p, Paths: []string{p}, Gone: true})
			}
		}
		devices = now
	})
}

func publishChange(h *hub.Hub, c watch.Change) {
	if _, err := h.PublishJSON(watch.EventChanged, hub.FolderTopic(c.Path), c); err != nil {
		logger.Errorf("publish %s: %v", c.Path, err)
	}
}
