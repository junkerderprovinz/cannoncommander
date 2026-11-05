package system

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Sources is what the sources page reads and saves. RestartNeeded is set
// while the saved sources differ from the ones the server runs with.
// Devices are the disks and pools Unraid reports, which admins see next to
// the sources without an index.
type Sources struct {
	Sources       []settings.StoredSource `json:"sources"`
	Devices       []Device                `json:"devices"`
	RestartNeeded bool                    `json:"restartNeeded"`
}

// Device is a disk or pool as the sources page lists it.
type Device struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *service) sources() (Sources, error) {
	out, err := s.configured()
	if err != nil {
		return Sources{}, err
	}
	out.Devices = []Device{}
	for _, src := range s.deps.FS.Sources() {
		if src.Device {
			out.Devices = append(out.Devices, Device{Name: src.Name, Path: src.Mount})
		}
	}
	return out, nil
}

func (s *service) configured() (Sources, error) {
	running := make([]settings.StoredSource, 0, len(s.running()))
	for _, src := range s.running() {
		running = append(running, settings.DescribeSource(src))
	}
	file := s.sourcesFile()
	if file == "" {
		return Sources{Sources: running}, nil
	}
	saved, ok, err := settings.ReadStoredSources(file)
	if err != nil {
		return Sources{}, err
	}
	if !ok {
		return Sources{Sources: running}, nil
	}
	for i := range saved {
		if saved[i].Exclude == nil {
			saved[i].Exclude = []string{}
		}
	}
	return Sources{Sources: saved, RestartNeeded: !settings.SameSources(saved, running)}, nil
}

func (s *service) saveSources(list []settings.StoredSource) (Sources, error) {
	if err := cleanSources(list); err != nil {
		return Sources{}, v1.Errorf(http.StatusBadRequest, err.Error())
	}
	file := s.sourcesFile()
	if file == "" {
		return Sources{}, fmt.Errorf("the place for saved sources is unknown")
	}
	if err := settings.WriteStoredSources(file, list); err != nil {
		return Sources{}, err
	}
	return s.sources()
}

// cleanSources checks the list the way the settings page does and tidies
// paths and patterns, so saving twice gives the same file.
func cleanSources(list []settings.StoredSource) error {
	if len(list) == 0 {
		return fmt.Errorf("keep at least one source")
	}
	names := map[string]bool{}
	paths := map[string]bool{}
	for i := range list {
		s := &list[i]
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" {
			return fmt.Errorf("give every source a name")
		}
		if names[strings.ToLower(s.Name)] {
			return fmt.Errorf("there is already a source called %s", s.Name)
		}
		names[strings.ToLower(s.Name)] = true

		p := strings.TrimSpace(s.Path)
		if !filepath.IsAbs(p) {
			return fmt.Errorf("%s: the folder needs a full path such as /mnt/user", s.Name)
		}
		s.Path = filepath.Clean(p)
		if info, err := os.Stat(s.Path); err != nil || !info.IsDir() {
			return fmt.Errorf("%s is not a folder on the server", s.Path)
		}
		if paths[s.Path] {
			return fmt.Errorf("%s is already a source", s.Path)
		}
		paths[s.Path] = true

		exclude := []string{}
		for _, raw := range s.Exclude {
			pat := strings.Trim(strings.TrimSpace(raw), "/")
			if pat == "" || slices.Contains(exclude, pat) {
				continue
			}
			if _, err := settings.ExcludeRule(pat); err != nil {
				return err
			}
			exclude = append(exclude, pat)
		}
		s.Exclude = exclude
	}
	return nil
}
